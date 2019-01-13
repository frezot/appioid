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

// State is an unexported type
type State struct {
	free bool
	dob  time.Time
}

var devicesPool = make(map[string]State)
var portsPool = make(map[string]State)
var bysyLimit = 180 * time.Second
var unsupportedOsError = "Sory, not implemented for UNIX systems yet"

func defaultAction(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,
		"You can use this commands \n"+
			"===========================================================\n"+
			"http://127.0.0.1:9090/getDevice\n"+
			"http://127.0.0.1:9090/releaseDevice?name={deviceName}\n"+
			"http://127.0.0.1:9090/getPort\n"+
			"http://127.0.0.1:9090/releasePort?port={number}\n"+
			"http://127.0.0.1:9090/rereadDevices\n"+
			"http://127.0.0.1:9090/forceCleanUp\n"+
			"===========================================================")
}

func debug(w http.ResponseWriter, r *http.Request) {
	log.Println(devicesPool)
	log.Println(portsPool)
}

func releaseDevice(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	target := ""
	for k, v := range r.Form {
		if k == "name" {
			target = v[0]
		}
	}
	if target == "" {
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n_host_/releaseDevice?name={deviceName}")
	} else {
		_, matched := devicesPool[target]
		if matched {
			devicesPool[target] = State{free: true, dob: time.Now()}
			fmt.Fprintf(w, "OK")
		} else {
			fmt.Fprintf(w, "UNKNOWN")
		}
	}
}

func releasePort(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	target := ""
	for k, v := range r.Form {
		if k == "port" {
			target = v[0]
		}
	}
	if target == "" {
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n_host_/releasePort?port={number}")
	} else {
		_, matched := portsPool[target]
		if matched {
			portsPool[target] = State{free: true, dob: time.Now()}
			cleanUpAppiumProcesses()
			fmt.Fprintf(w, "OK")
		} else {
			fmt.Fprintf(w, "UNKNOWN")
		}
	}
}

func getDevice(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, findFreeDevice())
}

func getPort(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, findFreePort())
}

func rereadDevices(w http.ResponseWriter, r *http.Request) {
	loadDevices()

	fmt.Fprintf(w, "Actual devices list:\n")
	fmt.Fprintf(w, "----------------------------\n")
	for d := range devicesPool {
		fmt.Fprintf(w, "| %-25s|\n", d)
	}
	fmt.Fprintf(w, "----------------------------")
}

func forceCleanUp(w http.ResponseWriter, r *http.Request) {
	cleanUpAppiumProcesses()
	loadDevices()
}

func findFreeDevice() string {
	//TODO: actualise list here in case of some changes on PC
	for name := range devicesPool {
		if devicesPool[name].free == true || time.Now().Sub(devicesPool[name].dob) > bysyLimit {
			devicesPool[name] = State{free: false, dob: time.Now()}
			return name
		}
	}
	return "PLEASE WAIT"
}

func findFreePort() string {
	cleanUpAppiumProcesses()
	for port := range portsPool {
		if portsPool[port].free == true {
			portsPool[port] = State{free: false, dob: time.Now()}
			return port
		} else if time.Now().Sub(portsPool[port].dob) > bysyLimit {
			portsPool[port] = State{free: true, dob: time.Now()}
			cleanUpAppiumProcesses()
		}
	}
	return "PLEASE WAIT"
}

func initialLoad() {
	//TODO: reads and apply portsPool size from env or args
	portsPool["4724"] = State{true, time.Now()}
	portsPool["4725"] = State{true, time.Now()}
	portsPool["4726"] = State{true, time.Now()}
	portsPool["4727"] = State{true, time.Now()}
	cleanUpAppiumProcesses()
	loadDevices()
}

func loadDevices() {
	cmd := exec.Command("adb", "devices")
	out, _ := cmd.CombinedOutput()
	cmdRes := string(out)
	r, _ := regexp.Compile(`(.*)\s+device\s`)
	devs := r.FindAllStringSubmatch(cmdRes, -1)

	adbListing := make(map[string]struct{})

	for _, elem := range devs {
		devName := elem[1] // first group from regexp match
		adbListing[devName] = struct{}{}

		_, registred := devicesPool[devName]
		// if detected device yet not registred in pool -> write down
		if !registred {
			devicesPool[devName] = State{true, time.Now()}
		}
	}

	for devFromPool := range devicesPool {
		_, online := adbListing[devFromPool]
		// if device were disconnected -> remove from pool
		if !online {
			delete(devicesPool, devFromPool)
		}
	}
}

func cleanUpAppiumProcesses() {
	for port := range portsPool {
		if portsPool[port].free == true {
			outlawPid := getProcessByPort(port)
			if outlawPid != "" {
				killPid(outlawPid)
			}
			//clean adb-processes too, they use another then appium port
			//numbering rule is simple: appium + 3500 (e.g. 4724 -> 8229)
			portAsInt, _ := strconv.Atoi(port)
			adbPort := strconv.Itoa(portAsInt + 3500)
			adbPid := getProcessByPort(adbPort)
			if adbPid != "" {
				killPid(adbPid)
			}
		}
	}
}

func getProcessByPort(p string) string {
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
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/F", "/pid", id)
		_, err := cmd.CombinedOutput()
		if err == nil {
			log.Println("[DONE] taskkill /pid " + id)
		}
	} else {
		log.Fatal(unsupportedOsError)
	}
}

func main() {

	http.HandleFunc("/", defaultAction)
	http.HandleFunc("/debug", debug)
	http.HandleFunc("/getDevice", getDevice)
	http.HandleFunc("/releaseDevice", releaseDevice)
	http.HandleFunc("/getPort", getPort)
	http.HandleFunc("/releasePort", releasePort)
	http.HandleFunc("/rereadDevices", rereadDevices)
	http.HandleFunc("/forceCleanUp", forceCleanUp)

	initialLoad()
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
