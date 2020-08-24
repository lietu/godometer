package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/lietu/godometer/monitor"
	"github.com/warthog618/gpiod/device/rpi"
)

var (
	dev                = flag.Bool("dev", false, "Development mode, enables profiler in port 8888. Optionally use the DEV environment variable.")
	device             = flag.String("device", "gpiochip0", "The /dev device name for GPIO to monitor. Optionally use the DEVICE environment variable.")
	pin                = flag.Int("pin", rpi.J8p11, "Which GPIO PIN to monitor. Optionally use the PIN environment variable.")
	wheelCircumference = flag.Float64("circumference", 0.2375, "Measurement wheel circumference in meters. Optionally use the WHEEL_CIRCUMFERENCE environment variable.")
	dbPath             = flag.String("db", "./godometer.txt", "Path to locally stored records. Optionally use the DB_PATH environment variable.")
	apiBaseUrl         = flag.String("apiBaseUrl", "http://localhost:8080", "API base URL where to report stats to, set as empty string to disable. Optionally use the API_BASE_URL environment variable.")
	apiAuth            = flag.String("apiAuth", "", "Password for API. Optionally use the API_AUTH environment variable.")
	quiet              = flag.Bool("quiet", false, "Stop reporting regular updates. Optionally use the QUIET environment variable.")
)

type Config struct {
	dev                bool
	device             string
	pin                int
	wheelCircumference float64
	dbPath             string
	apiBaseUrl         string
	apiAuth            string
	quiet              bool
}

func parseConfig() Config {
	flag.Parse()

	c := Config{
		dev:                *dev,
		device:             *device,
		pin:                *pin,
		wheelCircumference: *wheelCircumference,
		dbPath:             *dbPath,
		apiBaseUrl:         *apiBaseUrl,
		apiAuth:            *apiAuth,
		quiet:              *quiet,
	}

	if e := os.Getenv("DEVICE"); e != "" {
		c.device = e
	}

	if e := os.Getenv("PIN"); e != "" {
		i, err := strconv.Atoi(e)
		if err != nil {
			log.Printf("Could not parse PIN environment variable: %s", err)
		} else {
			c.pin = i
		}
	}

	if e := os.Getenv("WHEEL_CIRCUMFERENCE"); e != "" {
		f, err := strconv.ParseFloat(e, 64)
		if err != nil {
			log.Printf("Could not parse WHEEL_CIRCUMFERENCE environment variable: %s", err)
		} else {
			c.wheelCircumference = f
		}
	}

	if e := os.Getenv("DB_PATH"); e != "" {
		c.dbPath = e
	}

	if e := os.Getenv("API_BASE_URL"); e != "" {
		c.apiBaseUrl = e
	}

	if e := os.Getenv("API_AUTH"); e != "" {
		c.apiAuth = e
	}

	if e := os.Getenv("QUIET"); e != "" {
		if e == "1" || e == "yes" || e == "true" {
			c.quiet = true
		} else {
			c.quiet = false
		}
	}

	if e := os.Getenv("dev"); e != "" {
		if e == "1" || e == "yes" || e == "true" {
			c.dev = true
		} else {
			c.dev = false
		}
	}

	return c
}

func (c Config) Print() {
	pwd := "Not set"
	if c.apiAuth != "" {
		pwd = "Set"
	}

	log.Print(" ----- CONFIGURATION ----- ")
	log.Printf("Wheel circumference: %.5fm", c.wheelCircumference)

	log.Printf("Device:  %s", c.device)
	log.Printf("Pin:     %d", c.pin)
	log.Printf("DB path: %s", c.dbPath)

	log.Printf("API base URL: %s", c.apiBaseUrl)
	log.Printf("API pwd:      %s", pwd)
}

func main() {
	config := parseConfig()
	config.Print()

	exit := make(chan bool)
	exit2 := make(chan bool)
	results := make(chan monitor.GPIORecord, 100)

	gm := monitor.NewGPIOMonitor(config.device, config.pin, config.wheelCircumference, results)
	sm := monitor.NewStatsMonitor(results, config.dbPath, config.apiBaseUrl, config.apiAuth)

	go gm.Monitor(exit)
	go sm.Monitor(config.quiet, exit2)

	if config.dev {
		go func() {
			log.Println(http.ListenAndServe("0.0.0.0:8888", nil))
		}()
	}

	defer func() { exit <- true }()
	defer func() { exit2 <- true }()

	for {
		time.Sleep(10 * time.Second)
	}
}
