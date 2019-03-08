package devices

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/frezot/appioid/cmds"
	"github.com/frezot/appioid/settings"
)

type state struct {
	free bool
	port string // system port
	dob  time.Time
}

// PoolD map [name:state] + Mutex
type PoolD struct {
	sync.Mutex
	pool map[string]state
}

// NewPoolD smth like constructor
func NewPoolD() *PoolD {
	return &PoolD{pool: make(map[string]state)}
}

// PrintableStatus return formatted string about all devices
func (d *PoolD) PrintableStatus() string {
	result := ""
	for name, state := range d.pool {
		result += fmt.Sprintf("║ %-30s║ %-5t ║ %4s ║\n", name, state.free, state.port)
	}
	return result
}

// AllFree easy way to detect idle
func (d *PoolD) AllFree() bool {
	result := true
	for _, state := range d.pool {
		result = result && state.free
	}
	return result
}

// ForceCleanUp mark each device as free
func (d *PoolD) ForceCleanUp() {
	for name := range d.pool {
		//TODO remove uiAutomator
		d.SetFree(name)
	}
}

// SetFree method for change status to free and update time
func (d *PoolD) SetFree(name string) string {

	_, matched := d.pool[name]
	if matched {
		systemPort := d.pool[name].port
		// cmds.KillProcess(systemPort) //TODO: is it necessary??
		d.pool[name] = state{free: true, port: systemPort, dob: time.Now()}
		return "OK"
	}
	return "UNKNOWN"
}

// GetFree search for free device in pool and returns
func (d *PoolD) GetFree() string {
	d.Lock()
	defer d.Unlock()

	d.Refresh()

	for name := range d.pool {

		if !d.pool[name].free && time.Now().Sub(d.pool[name].dob) > settings.BusyLimit {
			log.Printf("[WARN] device '%s' TTL elapsed", name)
			d.SetFree(name)
			continue
		}
		if d.pool[name].free {
			systemPort := d.pool[name].port
			d.pool[name] = state{free: false, port: systemPort, dob: time.Now()}
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
		if devName == settings.ReservedDevice {
			continue
		}
		adbListing[devName] = struct{}{}

		_, registred := d.pool[devName]
		if !registred {
			d.pool[devName] = state{port: strconv.Itoa(settings.SystemPort), free: true, dob: time.Now()}
			settings.SystemPort++
		}
	}

	// stage2: find disconnected devices and remove from devicesPool
	for devFromPool := range d.pool {
		_, online := adbListing[devFromPool]
		if !online {
			cmds.KillProcess(d.pool[devFromPool].port)
			delete(d.pool, devFromPool)
		}
	}
}
