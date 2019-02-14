package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	appVersion         = "0.95"
	host               = "http://127.0.0.1"
	timeFormat         = "15:04:05"
	unsupportedOsError = "Sory, not implemented for UNIX systems yet 😰"
)

var (
	poolSize       int
	portCounter    int
	ttl            int
	appiodPort     string
	reservedDevice string
	busyLimit      time.Duration
)

// appiumState store short information about state
type appiumState struct {
	free bool
	dob  time.Time
}

// deviceState store short information about state + sysport
type deviceState struct {
	free bool
	port string
	dob  time.Time
}

// PoolA map [name:state] + Mutex
type PoolA struct {
	sync.Mutex
	pool map[string]appiumState
}

// PoolD map [name:state] + Mutex
type PoolD struct {
	sync.Mutex
	pool map[string]deviceState
}

var appiums = &PoolA{
	pool: make(map[string]appiumState),
}

var devices = &PoolD{
	pool: make(map[string]deviceState),
}

func defaultAction(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,
		"You can use this commands \n"+
			"===========================================================\n"+
			host+appiodPort+"/getDevice\n"+
			host+appiodPort+"/stopDevice?name={deviceName}\n"+
			"-----------------------------------------------------------\n"+
			host+appiodPort+"/getAppium\n"+
			host+appiodPort+"/stopAppium?port={number}\n"+
			"-----------------------------------------------------------\n"+
			host+appiodPort+"/status\n"+
			host+appiodPort+"/allFree\n"+
			"-----------------------------------------------------------\n"+
			host+appiodPort+"/forceCleanUp\n"+
			"===========================================================\n")
}

func getAppium(w http.ResponseWriter, r *http.Request) {
	res := appiums.GetFree()
	log.Printf("[DEBUG] /getAppium : %s", res)
	fmt.Fprintf(w, res)
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
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n\t%s%s/stopAppium?port={number}", host, appiodPort)
	} else {
		log.Printf("[DEBUG] /stopAppium?port=%s", target)
		fmt.Fprintf(w, appiums.SetFree(target))
	}
}

func getDevice(w http.ResponseWriter, r *http.Request) {
	res := devices.GetFree()
	log.Printf("[DEBUG] /getDevice : " + res)
	fmt.Fprintf(w, res)
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
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n\t%s%s/stopDevice?name={deviceName}", host, appiodPort)
	} else {
		log.Printf("[DEBUG] /stopDevice?name=%s", target)
		fmt.Fprintf(w, devices.SetFree(target))
	}
}

func status(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "⌚️ %s\n\nActual appiums list:\n", time.Now().Format(timeFormat))
	fmt.Fprintf(w, "|==============URL==============|=free?=|\n")
	for a := range appiums.pool {
		fmt.Fprintf(w, "| %-30s| %-5t | %s\n", appiumServerURL(a), appiums.pool[a].free, appiumStatus(a))
	}
	fmt.Fprintf(w, "-----------------------------------------\n\n")

	devices.Refresh()

	fmt.Fprintf(w, "Actual devices list:\n")
	fmt.Fprintf(w, "|=============NAME==============|=port=|=free?=|\n")
	for d := range devices.pool {
		fmt.Fprintf(w, "| %-30s| %4s | %-5t |\n", d, devices.pool[d].port, devices.pool[d].free)
	}
	fmt.Fprintf(w, "------------------------------------------------\n\n")
}

func allFree(w http.ResponseWriter, r *http.Request) {
	// [!] not thread safe
	result := true
	for d := range devices.pool {
		result = result && devices.pool[d].free
	}
	for a := range appiums.pool {
		result = result && appiums.pool[a].free
	}
	fmt.Fprintf(w, "%v", result)
}

func forceCleanUp(w http.ResponseWriter, r *http.Request) {

	begin := time.Now().Format(timeFormat)
	log.Printf("==> [START] Force restart <==")

	_, err := exec.Command("taskkill", "/F", "/IM", "node.exe").CombinedOutput()
	if err != nil {
		log.Println("[ERROR] Failed to start cmd: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(appiums.pool))
	for port := range appiums.pool {
		go func(p string) {
			defer wg.Done()
			appiums.Restart(p)
		}(port)
	}
	wg.Wait()

	for d := range devices.pool {
		devices.SetFree(d)
	}
	log.Printf("==> [FINISH] Force restart <==")
	end := time.Now().Format(timeFormat)
	fmt.Fprintf(w, "⏳\t%s\n⏰\t%s\n✅ forceCleanUp is complete\n", begin, end)
	status(w, r)
}

// SetFree method for change status to free and update time
func (a *PoolA) SetFree(name string) string {
	a.Lock()
	defer a.Unlock()

	_, matched := a.pool[name]
	if matched {
		a.pool[name] = appiumState{free: true, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN"
}

// GetFree search for free appium in pool and returns
func (a *PoolA) GetFree() string {
	a.Lock()
	defer a.Unlock()

	for port := range a.pool {
		if appiumStatus(port) == "ERR" {
			a.Restart(port)
			continue
		}
		if !a.pool[port].free && time.Now().Sub(a.pool[port].dob) > busyLimit {
			log.Printf("[WARN] appium:%s TTL elapsed", port)
			a.Restart(port)
			continue
		}
		if a.pool[port].free {
			a.pool[port] = appiumState{free: false, dob: time.Now()}
			return appiumServerURL(port)
		}
	}
	return "WAIT"
}

// SetFree method for change status to free and update time
func (d *PoolD) SetFree(name string) string {
	d.Lock()
	defer d.Unlock()

	_, matched := d.pool[name]
	if matched {
		systemPort := d.pool[name].port
		//killProcess(systemPort) //TODO: is it necessary??
		d.pool[name] = deviceState{free: true, port: systemPort, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN"
}

// GetFree search for free device in pool and returns
func (d *PoolD) GetFree() string {
	d.Lock()
	defer d.Unlock()

	d.Refresh()

	for name := range devices.pool {

		if !d.pool[name].free && time.Now().Sub(d.pool[name].dob) > busyLimit {
			log.Printf("[WARN] device '%s' TTL elapsed", name)
			d.SetFree(name)
			continue
		}
		if d.pool[name].free {
			systemPort := d.pool[name].port
			d.pool[name] = deviceState{free: false, port: systemPort, dob: time.Now()}
			return (name + " " + d.pool[name].port)
		}
	}
	return "WAIT"
}

// Refresh search for new devices and delete outdated
func (d *PoolD) Refresh() {
	out, _ := exec.Command("adb", "devices").CombinedOutput()
	exp, _ := regexp.Compile(`(.*)\s+device\s`)
	devs := exp.FindAllStringSubmatch(string(out), -1)

	adbListing := make(map[string]struct{})

	// stage1: find new devices and record to pool
	for _, elem := range devs {
		devName := elem[1] // first group from regexp match
		if devName == reservedDevice {
			continue
		}
		adbListing[devName] = struct{}{}

		_, registred := d.pool[devName]
		if !registred {
			d.pool[devName] = deviceState{port: strconv.Itoa(portCounter), free: true, dob: time.Now()}
			portCounter++
		}
	}

	// stage2: find disconnected devices and remove from devicesPool
	for devFromPool := range d.pool {
		_, online := adbListing[devFromPool]
		if !online {
			killProcess(d.pool[devFromPool].port)
			delete(d.pool, devFromPool)
		}
	}
}

// Restart kill (if exist) old process and start new one
func (a *PoolA) Restart(port string) {
	log.Printf("[DEBUG] restart %s", appiumServerURL(port))
	killProcess(port)
	startNode(port)
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

func appiumServerURL(port string) string {
	return host + ":" + port + "/wd/hub"
}

func startNode(port string) {

	if err := exec.Command("appium", "-p", port, "--log-level", "error").Start(); err == nil {
		if appiumIsReady(port) {
			log.Printf("[DONE] started %s", appiumServerURL(port))
			appiums.pool[port] = appiumState{free: true, dob: time.Now()}
		} else {
			log.Printf("[WARN] started %s but not responding", appiumServerURL(port))
			appiums.pool[port] = appiumState{free: false, dob: time.Now().Add(-1 * time.Hour)}
		}
	} else {
		log.Printf("[ERROR] Failed to start cmd: %v", err)
	}
}

func initialLoad() {
	var wg sync.WaitGroup
	wg.Add(poolSize)

	log.Printf("[INIT] Appioid started")
	for i := 0; i < poolSize; i++ {
		go func(num int) {
			defer wg.Done()
			startNode(strconv.Itoa(num))
		}(portCounter)
		portCounter++
	}
	wg.Wait()

	devices.Refresh()
}

func getPidByPort(p string) string {
	if runtime.GOOS == "windows" {
		out, _ := exec.Command("netstat", "-ano").CombinedOutput()
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
		_, err := exec.Command("taskkill", "/F", "/pid", id).CombinedOutput()
		if err == nil {
			log.Println("[DONE] taskkill /F /pid %s", id)
		}
	} else {
		log.Fatal(unsupportedOsError)
	}
}

func killProcess(port string) {
	killPid(getPidByPort(port))
}

func main() {
	version := flag.Bool("v", false, "Prints current appioid version")
	flag.StringVar(&appiodPort, "p", ":9093", "Port to listen on, don't forget colon at start")
	flag.StringVar(&reservedDevice, "rd", "", "Reserved device (never be returned by /getDevice)")
	flag.IntVar(&poolSize, "sz", 2, "How much appium servers should works at same time")
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
	http.HandleFunc("/allFree", allFree)
	http.HandleFunc("/forceCleanUp", forceCleanUp)

	initialLoad()

	log.Fatal(http.ListenAndServe(appiodPort, nil))

}
