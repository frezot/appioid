package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/frezot/appioid/manager"
	"github.com/frezot/appioid/settings"
	"github.com/frezot/appioid/utils"
)

const (
	appVersion = "0.98"
	timeFormat = "15:04:05"
)

var (
	ttl         int
	appioidPort string
	baseURL     string
)

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
	res := manager.Appiums.GetFree()
	log.Printf("[DBUG] /getAppium : %s", res)
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
		log.Printf("[DBUG] /stopAppium?port=%s", target)
		fmt.Fprintf(w, manager.Appiums.SetFree(target))
	}
}

func getDevice(w http.ResponseWriter, r *http.Request) {
	res := manager.Devices.GetFree()
	log.Println("[DBUG] /getDevice : ", res)
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
		log.Printf("[DBUG] /stopDevice?name=%s", target)
		fmt.Fprintf(w, manager.Devices.SetFree(target))
	}
}

func status(w http.ResponseWriter, r *http.Request) {

	fmt.Fprintf(w, "⌚️ %s\n\nActual appiums list:\n", time.Now().Format(timeFormat))
	fmt.Fprintf(w, "╔══════════════URL══════════════╦═free?═╗\n")
	fmt.Fprintf(w, manager.Appiums.PrintableStatus())
	fmt.Fprintf(w, "╚═══════════════════════════════╩═══════╝\n\n")

	manager.Devices.Refresh()

	fmt.Fprintf(w, "Actual devices list:\n")
	fmt.Fprintf(w, "╔═════════════NAME══════════════╦═free?═╦═port═╗\n")
	fmt.Fprintf(w, manager.Devices.PrintableStatus())
	fmt.Fprintf(w, "╚═══════════════════════════════╩═══════╩══════╝\n\n")
}

func allFree(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", manager.Devices.AllFree() && manager.Appiums.AllFree())
}

func forceCleanUp(w http.ResponseWriter, r *http.Request) {

	begin := time.Now().Format(timeFormat)
	log.Printf("==> [START] Force restart <==")
	manager.Appiums.ForceCleanUp()
	manager.Devices.ForceCleanUp()
	log.Printf("==> [FINISH] Force restart <==")
	end := time.Now().Format(timeFormat)

	fmt.Fprintf(w, "⏳\t%s\n⏰\t%s\n✅ forceCleanUp is complete\n", begin, end)
	status(w, r)
}

func main() {
	version := flag.Bool("v", false, "Prints current appioid version")
	flag.StringVar(&appioidPort, "p", "9093", "Port to listen on")
	flag.StringVar(&settings.ReservedDevice, "rd", "", "Reserved device (never be returned by /getDevice)")
	flag.IntVar(&settings.PoolSize, "sz", 2, "How much appium servers should works at same time")
	flag.IntVar(&settings.AppiumPort, "ap", 4725, "First value of appiumPort counter")
	flag.IntVar(&settings.SystemPort, "sp", 8202, "First value of systepPort counter")
	flag.IntVar(&ttl, "TTL", 300, "Max time (in seconds) which node or device might be in use")
	flag.Parse()

	if *version {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	baseURL = utils.BuildAppioidBaseURL(appioidPort)
	manager.Initialization(ttl)

	http.HandleFunc("/", defaultAction)
	http.HandleFunc("/getDevice", getDevice)
	http.HandleFunc("/stopDevice", stopDevice)
	http.HandleFunc("/getAppium", getAppium)
	http.HandleFunc("/stopAppium", stopAppium)
	http.HandleFunc("/status", status)
	http.HandleFunc("/allFree", allFree)
	http.HandleFunc("/forceCleanUp", forceCleanUp)

	log.Fatal(http.ListenAndServe(":"+appioidPort, nil))
}
