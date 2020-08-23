package monitor

import (
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"

	"github.com/warthog618/gpiod"
)

// These chosen based on min and max speeds to monitor and circumference of monitoring wheel
const (
	minElapsed = 40 * time.Millisecond   // 25x per second, so max 0.24*25 = 6m/s or 21.5km/h
	maxElapsed = 1500 * time.Millisecond // 0.66x per second, or 0.24 * 0.66 = 0.16m/s or 0.57km/h
)

type GPIORecord struct {
	Meters            float64
	MetersPerSecond   float64
	KilometersPerHour float64
}

type GPIOMonitor struct {
	device                   string
	pin                      int
	wheelCircumferenceMeters float64
	metersTraveled           float64
	lastRead                 time.Time
	lastValue                gpiod.LineEventType
	handlerMutex             *sync.Mutex
	results                  chan GPIORecord
}

func NewGPIOMonitor(device string, pin int, wheelCircumferenceMeters float64, results chan GPIORecord) *GPIOMonitor {
	gm := &GPIOMonitor{}
	gm.device = device
	gm.pin = pin
	gm.wheelCircumferenceMeters = wheelCircumferenceMeters
	gm.metersTraveled = 0.0
	gm.lastValue = gpiod.LineEventFallingEdge
	gm.results = results
	gm.handlerMutex = &sync.Mutex{}

	return gm
}

func metersPerSecond(elapsed time.Duration, circumferenceMeters float64) float64 {
	elapsedMillis := float64(elapsed) / float64(time.Millisecond)
	toSeconds := 1000.0 / elapsedMillis

	return toSeconds * circumferenceMeters
}

func (gm *GPIOMonitor) handler(evt gpiod.LineEvent) {
	gm.handlerMutex.Lock()
	defer gm.handlerMutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(gm.lastRead)
	value := evt.Type

	if gm.lastRead.IsZero() || elapsed > maxElapsed {
		// Reset counting whenever we've been paused for a little while
		gm.lastRead = now
		gm.lastValue = value
		return
	} else {
		// Too fast updates - some sort of flapping likely going on
		if elapsed < minElapsed {
			// fmt.Printf("-")
			return
		}
	}

	// Same values being sent repeatedly - junk data
	if value == gm.lastValue {
		// fmt.Printf(".")
		return
	}

	gm.lastRead = now
	gm.lastValue = value

	if value == gpiod.LineEventRisingEdge {
		mps := metersPerSecond(elapsed, gm.wheelCircumferenceMeters)
		kph := mps * 3600.0 / 1000.0 // 3600s/h & 1000m/km

		result := GPIORecord{
			Meters:            gm.wheelCircumferenceMeters,
			MetersPerSecond:   mps,
			KilometersPerHour: kph,
		}

		select {
		case gm.results <- result:
		default:
			log.Panic("Results buffer is full, something is very wrong!")
		}
	}
}

func (gm *GPIOMonitor) Monitor(exit chan bool) {
	// TODO: Report zero when not moving to make CLI output nicer
	// ... has minimal benefit for API usage, might actually make it worse

	c, err := gpiod.NewChip(gm.device)
	if err != nil {
		log.Panicf("Error opening chip %s: %s", gm.device, err)
	}
	defer func() {
		err := c.Close()
		if err != nil {
			log.Printf("Error closing chip: %s", err)
		}
	}()

	l, err := c.RequestLine(gm.pin, gpiod.AsInput, gpiod.WithBothEdges(gm.handler))
	if err != nil {
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPull* option requires kernel V5.5 or later - check your kernel version.")
		}
		log.Panicf("Error opening chip %s pin %d: %s", gm.device, gm.pin, err)
	}

	defer func() {
		err := l.Close()
		if err != nil {
			log.Printf("Error closing chip %s pin %s: %s", gm.device, gm.pin, err)
		}
	}()

	<-exit
}
