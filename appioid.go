package main

import "fmt"
import "net/http"
import "strings"
import "strconv"
import "log"
import "os/exec"
import "regexp"
import "time"
import "runtime"
import "io/ioutil"
import "flag"

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
var portCounter = 4724
var busyLimit = 180 * time.Second
var poolSize = 2
var reservedDevice = "WilNotBeUsedInPool"
var host = "http://127.0.0.1"
var appiodPort = ":9090"
var timeFormat = "15:04:05"
var unsupportedOsError = "Sory, not implemented for UNIX systems yet 😰"
var restartInProgress = false

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

func debug(w http.ResponseWriter, r *http.Request) {
	log.Println(devicesPool)
	log.Println(appiumsPool)
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

	fmt.Fprintf(w, "\nActual appiums list:\n")
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
	for port := range appiumsPool {
		tryToRestart(port)
	}
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
		adbPort := devicesPool[devicename].port
		killProcess(adbPort)
		devicesPool[devicename] = deviceState{free: true, port: adbPort, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN DEVICE NAME"
}

func deviceSetBusy(devicename string) {
	adbPort := devicesPool[devicename].port
	devicesPool[devicename] = deviceState{free: false, port: adbPort, dob: time.Now()}
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

	for i := 0; i < 7; i++ {
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
			tryToRestart(port)
			continue
		}
		if !appiumsPool[port].free && time.Now().Sub(appiumsPool[port].dob) > busyLimit {
			tryToRestart(port)
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
			log.Printf("[DONE] appium started on " + port)
			appiumsPool[port] = appiumState{true, time.Now()}
		} else {
			log.Printf("[ERROR] Failed to start appium on %s port", port)
			killProcess(port)
		}
	} else {
		log.Printf("[ERROR] Failed to start cmd: %v", err)
	}
}

func initialLoad() {

	for i := 0; i < poolSize; i++ {
		startAppiumNode(strconv.Itoa(portCounter))
		portCounter++
	}
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
		if time.Now().Sub(devicesPool[actualDevice].dob) > busyLimit {
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

func tryToRestart(appiumPort string) {
	if restartInProgress {
		log.Println("tryToRestart(" + appiumPort + ") failed. Locked")
	} else {
		restartInProgress = true
		killProcess(appiumPort)
		startAppiumNode(appiumPort)
		restartInProgress = false
	}
}

func main() {

	flag.IntVar(&poolSize, "s", 1, "How much appium servers should works at same time")
	flag.StringVar(&reservedDevice, "d", "", "You can set the device-name which shouldnt be used")
	flag.Parse()

	http.HandleFunc("/", defaultAction)
	http.HandleFunc("/debug", debug)
	http.HandleFunc("/getDevice", getDevice)
	http.HandleFunc("/stopDevice", stopDevice)

	http.HandleFunc("/getAppium", getAppium)
	http.HandleFunc("/stopAppium", stopAppium)

	http.HandleFunc("/status", status)
	http.HandleFunc("/forceCleanUp", forceCleanUp)

	initialLoad()
	err := http.ListenAndServe(appiodPort, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
