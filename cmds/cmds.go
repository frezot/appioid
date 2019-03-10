package cmds

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

const unsupportedOsError = "[FATAL] Unsupported OS: " + runtime.GOOS

// KillProcess find pid by LISTENING port and terminate
func KillProcess(port string) {
	killPid(getPidByPort(port))
}

func killPid(id string) {
	if id == "" {
		return // no pid -- no action
	}
	if runtime.GOOS == "windows" {
		_, err := exec.Command("taskkill", "/F", "/pid", id).CombinedOutput()
		if err == nil {
			log.Println("[DONE] taskkill /F /pid ", id)
		}
	} else if runtime.GOOS == "linux" {
		_, err := exec.Command("kill", id).CombinedOutput()
		if err == nil {
			log.Println("[DONE] kill ", id)
		}
	} else {
		log.Fatal(unsupportedOsError)
	}
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
	} else if runtime.GOOS == "linux" {
		out, _ := exec.Command("lsof", "-ti:"+p).CombinedOutput()
		return string(out)
	} else {
		log.Fatal(unsupportedOsError)
		return ""
	}
}

// WipeAppiumTools - remove appium relates apk-s from device
func WipeAppiumTools(d string) {
	result := fmt.Sprintf("device '%s'", d)

	if err := exec.Command("adb", "-s", d, "uninstall", "io.appium.uiautomator2.server").Start(); err != nil {
		result += fmt.Sprintf(" [ERR] %v", err)
	} else {
		result += " [OK]"
	}
	if err := exec.Command("adb", "-s", d, "uninstall", "io.appium.uiautomator2.server.test").Start(); err != nil {
		result += fmt.Sprintf(" [ERR] %v", err)
	} else {
		result += " [OK]"
	}
	if err := exec.Command("adb", "-s", d, "uninstall", "io.appium.settings").Start(); err != nil {
		result += fmt.Sprintf(" [ERR] %v", err)
	} else {
		result += " [OK]"
	}
	log.Printf(result)
}
