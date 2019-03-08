package appiums

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/frezot/appioid/cmds"
	"github.com/frezot/appioid/settings"
	"github.com/frezot/appioid/utils"
)

type state struct {
	free bool
	port string //bootstrap port
	dob  time.Time
}

// PoolA map [name:state] + Mutex
type PoolA struct {
	sync.Mutex
	pool map[string]state
}

// NewPoolA smth like constructor
func NewPoolA() *PoolA {
	return &PoolA{pool: make(map[string]state)}
}

// PrintableStatus return formatted string about all appiums
func (a *PoolA) PrintableStatus() string {
	result := ""
	for name, state := range a.pool {
		result += fmt.Sprintf("║ %-30s║ %-5t ║ %s\n", utils.AppiumServerURL(name), state.free, utils.AppiumStatus(name))
	}
	return result
}

// AllFree easy way to detect idle
func (a *PoolA) AllFree() bool {
	result := true
	for _, state := range a.pool {
		result = result && state.free
	}
	return result
}

// ForceCleanUp restart all known Appiums
func (a *PoolA) ForceCleanUp() {
	var wg sync.WaitGroup
	wg.Add(len(a.pool))
	for port := range a.pool {
		go func(p string) {
			defer wg.Done()
			a.restart(p)
		}(port)
	}
	wg.Wait()
}

// SetFree method for change status to free and update time
func (a *PoolA) SetFree(name string) string {
	_, matched := a.pool[name]
	if matched {
		a.Lock()
		defer a.Unlock()

		bp := a.pool[name].port
		a.pool[name] = state{free: true, port: bp, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN"
}

// GetFree search for free appium in pool and returns
func (a *PoolA) GetFree() string {
	a.Lock()
	defer a.Unlock()

	for port, st := range a.pool {
		if utils.AppiumStatus(port) == "ERR" {
			a.restart(port)
			continue
		}
		if !st.free && time.Now().Sub(st.dob) > settings.BusyLimit {
			log.Printf("[WARN] appium:%s TTL elapsed", port)
			a.restart(port)
			continue
		}
		if a.pool[port].free {
			bp := a.pool[port].port
			a.pool[port] = state{free: false, port: bp, dob: time.Now()}
			return utils.AppiumServerURL(port)
		}
	}
	return "WAIT"
}

func (a *PoolA) restart(p string) {
	cmds.KillProcess(p)
	bp := a.pool[p].port
	cmds.KillProcess(bp)
	a.StartNode(p, bp)
}

// StartNode start Appium server, check that ok, write log
func (a *PoolA) StartNode(port string, bp string) {

	if err := exec.Command("appium", "-p", port, "-bp", bp, "--log-level", "error", "--session-override").Start(); err == nil {
		if utils.AppiumIsReady(port) {
			log.Printf("[DONE] started %s", utils.AppiumServerURL(port))
			a.pool[port] = state{free: true, port: bp, dob: time.Now()}
		} else {
			log.Printf("[WARN] started %s but not responding", utils.AppiumServerURL(port))
			a.pool[port] = state{free: false, port: bp, dob: time.Now().Add(-1 * time.Hour)}
		}
	} else {
		log.Printf("[ERROR] Failed to start cmd: %v", err)
	}
}
