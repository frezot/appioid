package cmds

import (
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

const unsupportedOsError = "Sory, not implemented for UNIX systems yet 😰"

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
	}
	log.Fatal(unsupportedOsError)
	return ""
}
