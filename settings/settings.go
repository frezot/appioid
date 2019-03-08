package settings

import "time"

var (
	PoolSize       int
	AppiumPort     int
	SystemPort     int // http://appium.io/docs/en/writing-running-appium/caps/
	ReservedDevice string
	BusyLimit      time.Duration
)
