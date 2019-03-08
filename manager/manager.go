package manager

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/frezot/appioid/manager/appiums"
	"github.com/frezot/appioid/manager/devices"
	"github.com/frezot/appioid/settings"
)

// Devices can manage pool of Android devices
var Devices = devices.NewPoolD()

// Appiums can manage pool of Appium servers
var Appiums = appiums.NewPoolA()

// Initialization do everything that should happend on start
func Initialization(ttl int) {

	settings.BusyLimit = time.Duration(ttl) * time.Second

	log.Printf("[INIT] Appioid started")

	Devices.Refresh()

	var wg sync.WaitGroup
	wg.Add(settings.PoolSize)
	for i := 0; i < settings.PoolSize; i++ {
		go func(num int) {
			defer wg.Done()
			Appiums.StartNode(strconv.Itoa(num), strconv.Itoa(num+1))
		}(settings.AppiumPort)
		settings.AppiumPort += 2
	}
	wg.Wait()
}
