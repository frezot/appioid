package utils

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/frezot/appioid/settings"
)

const localhost = "http://127.0.0.1:"

// BuildAppioidBaseURL detects ip and concat with port to URL
func BuildAppioidBaseURL(port string) string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Println("[DEBUG] ip autodetect failed. localhost will be used")
		return localhost + port
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return "http://" + localAddr.IP.String() + ":" + port
}

// AppiumServerURL build URL for interact with appium
func AppiumServerURL(port string) string {
	return localhost + port + "/wd/hub"
}

// AppiumStatus return current atatus of Appium server
func AppiumStatus(port string) string {
	response, err := http.Get(AppiumServerURL(port) + "/status")
	if err == nil {
		defer response.Body.Close()
		responseData, _ := ioutil.ReadAll(response.Body)
		return string(responseData)
	}
	return "ERR"
}

// AppiumIsReady gently get status and returns as boolean
func AppiumIsReady(port string) bool {

	singleLatency := 6 //experimentally established value

	for i := 0; i < singleLatency*settings.PoolSize; i++ {
		if AppiumStatus(port) == "ERR" {
			time.Sleep(500 * time.Millisecond)
		} else {
			return true
		}
	}
	return (AppiumStatus(port) != "ERR")
}
