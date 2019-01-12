package main

import "fmt"
import "net/http"
import "strings"
import "log"
import "os/exec"
import "regexp"
import "runtime"

func defaultAction(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, 
        "Yo can use this commands \n" +
        "===========================================================\n" +
        "http://127.0.0.1:9090/getDevice\n" +
        "http://127.0.0.1:9090/releaseDevice?name={deviceName}\n" +
        "http://127.0.0.1:9090/getPort\n" +
        "http://127.0.0.1:9090/releasePort?port={number}\n" +
        "http://127.0.0.1:9090/rereadDevices\n" +
        "http://127.0.0.1:9090/forceCleanUp\n" +
        "===========================================================")
}

func releaseDevice(w http.ResponseWriter, r *http.Request) {
    //TODO: implement
}

func releasePort(w http.ResponseWriter, r *http.Request) {
    //TODO: implement
}

func getDevice(w http.ResponseWriter, r *http.Request) {
    //TODO: implement
}

func getPort(w http.ResponseWriter, r *http.Request) {
    //TODO: implement
}

func rereadDevices(w http.ResponseWriter, r *http.Request) {
    //TODO: loadDevices
    //TODO: print info
}

func forceCleanUp(w http.ResponseWriter, r *http.Request) {
    //TODO: cleanUpAppiumProcesses
    //TODO: loadDevices
}

func findFreeDevice() string {
    //TODO: actualise list here in case of some changes on PC
    return "not implemented"
}

func initialLoad() {
    //TODO: cleanUpAppiumProcesses
    //TODO: loadDevices
}

func loadDevices() {
    //TODO: adb devices -> find names -> store in poll
}


func cleanUpAppiumProcesses() {
    //TODO: search by ports and kill by pids
}

func getProcessByPort(p string) string {
    if runtime.GOOS == "windows" {
        cmd := exec.Command("netstat", "-ano")
        out, _ := cmd.CombinedOutput()
        pattern := strings.Join([]string{`TCP.*[:]`, p, `\s+.*LISTENING\s+(\d+)`}, "")
        r, _ := regexp.Compile(pattern)
    
        if len(r.FindStringSubmatch(string(out))) == 0 { return "" }
        pid := r.FindStringSubmatch(string(out))[1]
        return pid
    } else {
        log.Fatal("Not implemented for UNIX systems yet")
        return "ERROR"
    }
}

func killPid(id string) {
    if runtime.GOOS == "windows" {
        cmd := exec.Command("taskkill", "/F", "/pid", id)
        _, err := cmd.CombinedOutput()
        if err == nil { log.Println("[DONE] taskkill /pid " + id) }
    } else {
        log.Fatal("Not implemented for UNIX systems yet")
    }
}

func main() {

    http.HandleFunc("/",              defaultAction)
    http.HandleFunc("/getDevice",     getDevice)
    http.HandleFunc("/releaseDevice", releaseDevice)
    http.HandleFunc("/getPort",       getPort)
    http.HandleFunc("/releasePort",   releasePort)
    http.HandleFunc("/rereadDevices", rereadDevices)
    http.HandleFunc("/forceCleanUp",  forceCleanUp)


    initialLoad()
    err := http.ListenAndServe(":9090", nil)
    if err != nil { log.Fatal("ListenAndServe: ", err) }
}
