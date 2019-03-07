package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/frezot/appioid/cmds"
	"github.com/frezot/appioid/manager"
	"github.com/frezot/appioid/utils"
)

const (
	appVersion         = "0.96"
	timeFormat         = "15:04:05"
	unsupportedOsError = "Sory, not implemented for UNIX systems yet 😰"
)

var (
	poolSize    int
	appiumPort  int
	ttl         int
	baseURL     string
	appioidPort string
	busyLimit   time.Duration
)

// State store short information about state of appium or device
type State struct {
	free bool
	port string // bootstrap for appium, system for device
	dob  time.Time
}

// PoolA map [name:state] + Mutex
type PoolA struct {
	sync.Mutex
	pool map[string]State
}

var appiums = &PoolA{pool: make(map[string]State)}

func defaultAction(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,
		"You can use this commands \n"+
			"═══════════════════════════════════════════════════════════\n"+
			" "+baseURL+"/getDevice\n"+
			" "+baseURL+"/stopDevice?name={deviceName}\n"+
			"───────────────────────────────────────────────────────────\n"+
			" "+baseURL+"/getAppium\n"+
			" "+baseURL+"/stopAppium?port={number}\n"+
			"───────────────────────────────────────────────────────────\n"+
			" "+baseURL+"/status\n"+
			" "+baseURL+"/allFree\n"+
			"───────────────────────────────────────────────────────────\n"+
			" "+baseURL+"/forceCleanUp\n"+
			"═══════════════════════════════════════════════════════════\n")
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
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n\t%s/stopAppium?port={number}", baseURL)
	} else {
		log.Printf("[DEBUG] /stopAppium?port=%s", target)
		fmt.Fprintf(w, appiums.SetFree(target))
	}
}

func getDevice(w http.ResponseWriter, r *http.Request) {
	res := manager.Devices.GetFree()
	log.Println("[DEBUG] /getDevice : ", res)
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
		fmt.Fprintf(w, "INCORRECT REQUEST \nExpected form: \n\t%s/stopDevice?name={deviceName}", baseURL)
	} else {
		log.Printf("[DEBUG] /stopDevice?name=%s", target)
		fmt.Fprintf(w, manager.Devices.SetFree(target))
	}
}

func status(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "⌚️ %s\n\nActual appiums list:\n", time.Now().Format(timeFormat))
	fmt.Fprintf(w, "╔══════════════URL══════════════╦═free?═╗\n")
	for a := range appiums.pool {
		fmt.Fprintf(w, "║ %-30s║ %-5t ║ %s\n", utils.AppiumServerURL(a), appiums.pool[a].free, utils.AppiumStatus(a))
	}
	fmt.Fprintf(w, "╚═══════════════════════════════╩═══════╝\n\n")

	manager.Devices.Refresh()

	fmt.Fprintf(w, "Actual devices list:\n")
	fmt.Fprintf(w, "╔═════════════NAME══════════════╦═free?═╦═port═╗\n")
	fmt.Fprintf(w, manager.Devices.PrintableStatus())
	fmt.Fprintf(w, "╚═══════════════════════════════╩═══════╩══════╝\n\n")
}

func allFree(w http.ResponseWriter, r *http.Request) {
	// [!] not thread safe
	// result := true

	// for a := range appiums.pool {
	// 	result = result && appiums.pool[a].free
	// }
	fmt.Fprintf(w, "%v", manager.Devices.AllFree())
}

func forceCleanUp(w http.ResponseWriter, r *http.Request) {

	begin := time.Now().Format(timeFormat)
	log.Printf("==> [START] Force restart <==")

	_, err := exec.Command("taskkill", "/F", "/IM", "node.exe").CombinedOutput()
	if err != nil {
		log.Printf("[ERROR] Failed to start cmd: %v\n", err)
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

	manager.Devices.ForceCleanUp()

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
		bp := a.pool[name].port
		a.pool[name] = State{free: true, port: bp, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN"
}

// GetFree search for free appium in pool and returns
func (a *PoolA) GetFree() string {
	a.Lock()
	defer a.Unlock()

	for port := range a.pool {
		if utils.AppiumStatus(port) == "ERR" {
			a.Restart(port)
			continue
		}
		if !a.pool[port].free && time.Now().Sub(a.pool[port].dob) > busyLimit {
			log.Printf("[WARN] appium:%s TTL elapsed", port)
			a.Restart(port)
			continue
		}
		if a.pool[port].free {
			bp := a.pool[port].port
			a.pool[port] = State{free: false, port: bp, dob: time.Now()}
			return utils.AppiumServerURL(port)
		}
	}
	return "WAIT"
}

// Restart will kill (if exist) old process and start new one
func (a *PoolA) Restart(p string) {
	log.Printf("[DEBUG] restart %s", utils.AppiumServerURL(p))
	cmds.KillProcess(p)
	bp := a.pool[p].port
	cmds.KillProcess(bp)
	startNode(p, bp)
}

func appiumIsReady(port string) bool {

	singleLatency := 6 //experimentally established value

	for i := 0; i < singleLatency*poolSize; i++ {
		if utils.AppiumStatus(port) == "ERR" {
			time.Sleep(500 * time.Millisecond)
		} else {
			return true
		}
	}
	return (utils.AppiumStatus(port) != "ERR")
}

func startNode(port string, bp string) {

	if err := exec.Command("appium", "-p", port, "-bp", bp, "--log-level", "error", "--session-override").Start(); err == nil {
		if appiumIsReady(port) {
			log.Printf("[DONE] started %s", utils.AppiumServerURL(port))
			appiums.pool[port] = State{free: true, port: bp, dob: time.Now()}
		} else {
			log.Printf("[WARN] started %s but not responding", utils.AppiumServerURL(port))
			appiums.pool[port] = State{free: false, port: bp, dob: time.Now().Add(-1 * time.Hour)}
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
			startNode(strconv.Itoa(num), strconv.Itoa(num+1))
		}(appiumPort)
		appiumPort += 2
	}
	wg.Wait()

	manager.Devices.Refresh()
}

func main() {
	version := flag.Bool("v", false, "Prints current appioid version")
	flag.StringVar(&appioidPort, "p", "9093", "Port to listen on")
	flag.StringVar(&manager.ReservedDevice, "rd", "", "Reserved device (never be returned by /getDevice)")
	flag.IntVar(&poolSize, "sz", 2, "How much appium servers should works at same time")
	flag.IntVar(&appiumPort, "ap", 4725, "First value of appiumPort counter")

	flag.IntVar(&manager.SystemPort, "sp", 8202, "First value of systepPort counter")
	flag.IntVar(&ttl, "TTL", 300, "Max time (in seconds) which node or device might be in use")
	flag.Parse()

	if *version {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	busyLimit = time.Duration(ttl) * time.Second
	manager.BusyLimit = time.Duration(ttl) * time.Second

	baseURL = utils.BuildAppioidBaseURL(appioidPort)

	http.HandleFunc("/", defaultAction)

	http.HandleFunc("/getDevice", getDevice)
	http.HandleFunc("/stopDevice", stopDevice)

	http.HandleFunc("/getAppium", getAppium)
	http.HandleFunc("/stopAppium", stopAppium)

	http.HandleFunc("/status", status)
	http.HandleFunc("/allFree", allFree)
	http.HandleFunc("/forceCleanUp", forceCleanUp)

	initialLoad()

	log.Fatal(http.ListenAndServe(":"+appioidPort, nil))

}
