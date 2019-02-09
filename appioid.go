package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/netutil"
)

// appVersion is the current application version
const appVersion = "0.93"

// appiumState is an unexported type
type appiumState struct {
	free bool
	dob  time.Time
}

// deviceState is an unexported type
type deviceState struct {
	free bool
	port string
	dob  time.Time
}

var devicesPool = make(map[string]deviceState)
var appiumsPool = make(map[string]appiumState)

var poolSize, portCounter, ttl int
var appiodPort, reservedDevice string
var busyLimit time.Duration

var host = "http://127.0.0.1"
var timeFormat = "15:04:05"
var unsupportedOsError = "Sory, not implemented for UNIX systems yet 😰"

func defaultAction(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,
		"You can use this commands \n"+
			"===========================================================\n"+
			host+appiodPort+"/getDevice\n"+
			host+appiodPort+"/stopDevice?name={deviceName}\n"+

			host+appiodPort+"/getAppium\n"+
			host+appiodPort+"/stopAppium?port={number}\n"+

			host+appiodPort+"/status\n"+
			host+appiodPort+"/forceCleanUp\n"+
			"===========================================================")
}

func getDevice(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, discoverFreeDevice())
}

func stopDevice(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	target := ""
	for k, v := range r.Form {
		if k == "name" {
			target = v[0]
		}
	}
	if target == "" {
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n_host_/stopDevice?name={deviceName}")
	} else {
		fmt.Fprintf(w, deviceSetFree(target))
	}
}

func getAppium(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, discoverFreeAppium())
}

func stopAppium(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	target := ""
	for k, v := range r.Form {
		if k == "port" {
			target = v[0]
		}
	}
	if target == "" {
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n_host_/stopAppium?port={number}")
	} else {
		fmt.Fprintf(w, appiumSetFree(target))
	}
}

func status(w http.ResponseWriter, r *http.Request) {
	loadDevices()

	fmt.Fprintf(w, "⌚️ %s\n\nActual appiums list:\n", time.Now().Format(timeFormat))
	fmt.Fprintf(w, "|==============URL==============|=free?=|\n")
	for a := range appiumsPool {
		fmt.Fprintf(w, "| %-30s| %-5t | %s\n", appiumServerURL(a), appiumsPool[a].free, appiumStatus(a))
	}
	fmt.Fprintf(w, "-----------------------------------------\n\n")

	fmt.Fprintf(w, "Actual devices list:\n")
	fmt.Fprintf(w, "|=============NAME==============|=port=|=free?=|\n")
	for d := range devicesPool {
		fmt.Fprintf(w, "| %-30s| %4s | %-5t |\n", d, devicesPool[d].port, devicesPool[d].free)
	}
	fmt.Fprintf(w, "------------------------------------------------\n\n")
}

func forceCleanUp(w http.ResponseWriter, r *http.Request) {

	begin := time.Now().Format(timeFormat)

	log.Printf("==> [START] Force restart <==")

	var wg sync.WaitGroup
	wg.Add(len(appiumsPool))
	for port := range appiumsPool {
		go func(p string) {
			defer wg.Done()
			restart(p)
		}(port)
	}
	wg.Wait()

	loadDevices()
	for d := range devicesPool {
		deviceSetFree(d)
	}
	log.Printf("==> [FINISH] Force restart <==")
	end := time.Now().Format(timeFormat)
	fmt.Fprintf(w, "⏳\t%s\n⏰\t%s\n✅ forceCleanUp is complete\n", begin, end)
	status(w, r)
}

func appiumSetFree(appiumname string) string {
	_, matched := appiumsPool[appiumname]
	if matched {
		appiumsPool[appiumname] = appiumState{free: true, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN PORT"
}

func appiumSetBusy(key string) {
	appiumsPool[key] = appiumState{free: false, dob: time.Now()}
}

func deviceSetFree(devicename string) string {
	_, matched := devicesPool[devicename]
	if matched {
		systemPort := devicesPool[devicename].port
		killProcess(systemPort)
		devicesPool[devicename] = deviceState{free: true, port: systemPort, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN DEVICE NAME"
}

func deviceSetBusy(devicename string) {
	systemPort := devicesPool[devicename].port
	devicesPool[devicename] = deviceState{free: false, port: systemPort, dob: time.Now()}
}

func appiumStatus(port string) string {
	response, err := http.Get(appiumServerURL(port) + "/status")
	if err == nil {
		defer response.Body.Close()
		responseData, _ := ioutil.ReadAll(response.Body)
		return string(responseData)
	}
	return "ERR"
}

func appiumIsReady(port string) bool {

	singleLatency := 6 //experimentally established value

	for i := 0; i < singleLatency*poolSize; i++ {
		if appiumStatus(port) == "ERR" {
			time.Sleep(500 * time.Millisecond)
		} else {
			return true
		}
	}
	return (appiumStatus(port) != "ERR")
}

func discoverFreeDevice() string {
	loadDevices()
	for name := range devicesPool {
		if devicesPool[name].free {
			deviceSetBusy(name)
			return (name + " " + devicesPool[name].port)
		}
	}
	return "WAIT"
}

func discoverFreeAppium() string {
	for port := range appiumsPool {
		if appiumStatus(port) == "ERR" {
			restart(port)
			continue
		}
		if !appiumsPool[port].free && time.Now().Sub(appiumsPool[port].dob) > busyLimit {
			log.Printf("[WARN] appium:%s TTL elapsed", port)
			restart(port)
			continue
		}
		if appiumsPool[port].free {
			appiumSetBusy(port)
			return appiumServerURL(port)
		}
	}
	return "WAIT"
}

func appiumServerURL(port string) string {
	return host + ":" + port + "/wd/hub"
}

func startAppiumNode(port string) {
	cmd := exec.Command("appium", "-p", port, "--log-level", "error")
	if err := cmd.Start(); err == nil {
		if appiumIsReady(port) {
			log.Printf("[DONE] appium:%s started", port)
			appiumsPool[port] = appiumState{free: true, dob: time.Now()}
		} else {
			log.Printf("[ERROR] appium:%s started but not responding", port)
			appiumsPool[port] = appiumState{free: false, dob: time.Now().Add(-1 * time.Hour)}
		}
	} else {
		log.Printf("[ERROR] Failed to start cmd: %v", err)
	}
}

func initialLoad() {

	log.Printf("[INIT] appioid started")

	var wg sync.WaitGroup
	wg.Add(poolSize)
	for i := 0; i < poolSize; i++ {
		go func(num int) {
			defer wg.Done()
			startAppiumNode(strconv.Itoa(num))
		}(portCounter)
		portCounter++
	}
	wg.Wait()

	loadDevices()
}

func loadDevices() {
	cmd := exec.Command("adb", "devices")
	out, _ := cmd.CombinedOutput()
	cmdRes := string(out)
	r, _ := regexp.Compile(`(.*)\s+device\s`)
	devs := r.FindAllStringSubmatch(cmdRes, -1)

	adbListing := make(map[string]struct{})

	// stage1: find new devices and record to devicesPool
	for _, elem := range devs {
		devName := elem[1] // first group from regexp match
		if devName == reservedDevice {
			continue
		}
		adbListing[devName] = struct{}{}

		_, registred := devicesPool[devName]
		if !registred {
			devicesPool[devName] = deviceState{port: strconv.Itoa(portCounter), free: true, dob: time.Now()}
			portCounter++
		}
	}

	// stage2: find disconnected devices and remove from devicesPool
	for devFromPool := range devicesPool {
		_, online := adbListing[devFromPool]
		if !online {
			delete(devicesPool, devFromPool)
		}
	}

	// stage3: check for devices which is busy more than busyLimit
	for actualDevice := range devicesPool {
		if !devicesPool[actualDevice].free && time.Now().Sub(devicesPool[actualDevice].dob) > busyLimit {
			log.Printf("[WARN] device '%s' TTL elapsed", actualDevice)
			deviceSetFree(actualDevice)
		}
	}
}

func getPidByPort(p string) string {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("netstat", "-ano")
		out, _ := cmd.CombinedOutput()
		pattern := strings.Join([]string{`TCP.*[:]`, p, `\s+.*LISTENING\s+(\d+)`}, "")
		r, _ := regexp.Compile(pattern)

		if len(r.FindStringSubmatch(string(out))) == 0 {
			return ""
		}
		pid := r.FindStringSubmatch(string(out))[1]
		return pid
	}
	log.Fatal(unsupportedOsError)
	return ""
}

func killPid(id string) {
	if id == "" {
		return // no pid -- no action
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/F", "/pid", id)
		_, err := cmd.CombinedOutput()
		if err == nil {
			log.Println("[DONE] taskkill /F /pid " + id)
		}
	} else {
		log.Fatal(unsupportedOsError)
	}
}

func killProcess(port string) {
	killPid(getPidByPort(port))
}

func restart(appiumPort string) {
	killProcess(appiumPort)
	startAppiumNode(appiumPort)
}

func main() {

	version := flag.Bool("v", false, "Prints current appioid version")
	flag.StringVar(&appiodPort, "p", ":9093", "Port to listen on, don't forget colon at start")
	flag.StringVar(&reservedDevice, "rd", "", "Reserved device (never be returned by /getDevice)")
	flag.IntVar(&poolSize, "sz", 1, "How much appium servers should works at same time")
	flag.IntVar(&portCounter, "first", 4725, "First value of portCounter")
	flag.IntVar(&ttl, "TTL", 300, "Max time (in seconds) which node or device might be in use")
	flag.Parse()

	if *version {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	busyLimit = time.Duration(ttl) * time.Second

	http.HandleFunc("/", defaultAction)
	http.HandleFunc("/getDevice", getDevice)
	http.HandleFunc("/stopDevice", stopDevice)

	http.HandleFunc("/getAppium", getAppium)
	http.HandleFunc("/stopAppium", stopAppium)

	http.HandleFunc("/status", status)
	http.HandleFunc("/forceCleanUp", forceCleanUp)

	initialLoad()

	flow, err := net.Listen("tcp", appiodPort)
	if err != nil {
		log.Fatal("Listen: ", err)
	}
	defer flow.Close()

	flow = netutil.LimitListener(flow, 1)

	log.Fatal(http.Serve(flow, nil))

}
