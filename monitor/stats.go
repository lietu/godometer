package monitor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/lietu/godometer"
)

const statsDebug = false

// Keep this many measurements and average m/s and km/h over them for less variation
const averageOverMeasurements = 3

// Store and send this many dataPoints to ensure they get eventually delivered
const keepPastDataPoints = 5

var utc, _ = time.LoadLocation("UTC")

type FileDataPoint struct {
	Timestamp         string  `json:"ts"`
	Meters            float32 `json:"m"`
	MetersPerSecond   float32 `json:"mps"`
	KilometersPerHour float32 `json:"kph"`
	TotalMeters       float64 `json:"tm"`
}

func (fdp FileDataPoint) toAPIDataPoint() godometer.UpdateDataPoint {
	return godometer.UpdateDataPoint{
		Timestamp:         fdp.Timestamp,
		Meters:            fdp.Meters,
		MetersPerSecond:   fdp.MetersPerSecond,
		KilometersPerHour: fdp.KilometersPerHour,
	}
}

type StatsData struct {
	GPIORecords []GPIORecord
	dataPoints  []FileDataPoint
}

func NewStatsData() StatsData {
	return StatsData{
		GPIORecords: []GPIORecord{},
		dataPoints:  []FileDataPoint{},
	}
}

type StatsMonitor struct {
	results             chan GPIORecord
	apiBaseUrl          string
	apiAuth             string
	dbPath              string
	metersTraveled      float64
	totalMetersTraveled float64
	currentMPS          float64
	currentKPH          float64
	averageResults      []GPIORecord
	stats               StatsData
	statsMutex          *sync.Mutex
}

func NewStatsMonitor(results chan GPIORecord, dbPath string, apiBaseUrl string, apiAuth string) *StatsMonitor {
	sm := &StatsMonitor{}
	sm.results = results
	sm.dbPath = dbPath
	sm.apiBaseUrl = apiBaseUrl
	sm.apiAuth = apiAuth
	sm.metersTraveled = 0.0
	sm.totalMetersTraveled = 0.0
	sm.currentMPS = 0.0
	sm.currentKPH = 0.0
	sm.stats = NewStatsData()
	sm.statsMutex = &sync.Mutex{}
	sm.readLocalDB()
	return sm
}

func (sm *StatsMonitor) update(result GPIORecord) {
	results := len(sm.averageResults) + 1
	totalMPS := result.MetersPerSecond
	totalKPH := result.KilometersPerHour

	for _, r := range sm.averageResults {
		totalMPS += r.MetersPerSecond
		totalKPH += r.KilometersPerHour
	}

	currentMPS := totalMPS / float64(results)
	currentKPH := totalKPH / float64(results)

	keepFrom := 0
	if results > averageOverMeasurements {
		keepFrom = results - averageOverMeasurements
	}

	sm.averageResults = append(sm.averageResults, result)[keepFrom:]

	newRecord := GPIORecord{
		Meters:            result.Meters,
		MetersPerSecond:   currentMPS,
		KilometersPerHour: currentKPH,
	}

	// Update live stats
	sm.totalMetersTraveled += result.Meters
	sm.currentMPS = currentMPS
	sm.currentKPH = currentKPH

	// Then the periodical stats that require mutexing
	sm.statsMutex.Lock()
	defer sm.statsMutex.Unlock()

	sm.metersTraveled += result.Meters
	sm.stats.GPIORecords = append(sm.stats.GPIORecords, newRecord)
}

func (sm *StatsMonitor) readLocalDB() {
	if _, err := os.Stat(sm.dbPath); err != nil {
		if os.IsNotExist(err) {
			// No old data yet, this is fine
			log.Printf("No old data found from %s", sm.dbPath)
			return
		}

		// Something else went wrong, this might not be fine.
		log.Printf("Uh oh, could not read %s: %s", sm.dbPath, err)
		return
	}

	file, err := os.Open(sm.dbPath)
	if err != nil {
		log.Printf("Uh oh, could not read %s: %s", sm.dbPath, err)
		return
	}

	defer func() {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing %s: %s", sm.dbPath, err)
		}
	}()

	scanner := bufio.NewScanner(file)
	lineno := 0
	for scanner.Scan() {
		lineno += 1
		line := scanner.Bytes()

		fdp := FileDataPoint{}
		err := json.Unmarshal(line, &fdp)
		if err != nil {
			log.Printf("Error reading %s line %d: %s", sm.dbPath, lineno, err)
			continue
		}

		if fdp.TotalMeters > sm.totalMetersTraveled {
			sm.totalMetersTraveled = fdp.TotalMeters
		}

		sm.stats.dataPoints = append(sm.stats.dataPoints, fdp)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error parsing old data from %s: %s", sm.dbPath, err)
	}

	keepFrom := 0
	rows := len(sm.stats.dataPoints)
	if rows > keepPastDataPoints {
		keepFrom = rows - keepPastDataPoints
	}

	sm.stats.dataPoints = sm.stats.dataPoints[keepFrom:]
	log.Printf("Read %d old records from %s", len(sm.stats.dataPoints), sm.dbPath)
}

func (sm *StatsMonitor) writeLocalDB(rows []FileDataPoint) {
	contents := ""
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			// Fingers crossed, maybe this will fix itself.
			log.Printf("Could not marshal row: %s. This should not happen.", err)
			return
		}

		contents += string(data[:]) + "\n"
	}

	err := ioutil.WriteFile(sm.dbPath, []byte(contents), 0600)
	if err != nil {
		// Can't write to disk. Let's try to not panic and pretend someone will fix this.
		log.Printf("Could not write to %s: %s", sm.dbPath, err)
		return
	}
}

func (sm *StatsMonitor) saveStats() {
	// Get latest stats and replace container
	sm.statsMutex.Lock()

	records := float64(len(sm.stats.GPIORecords))

	recordMeters := 0.0
	totalMPS := 0.0
	totalKPH := 0.0
	avgMPS := 0.0
	avgKPH := 0.0

	for _, r := range sm.stats.GPIORecords {
		recordMeters += r.Meters
		totalMPS += r.MetersPerSecond
		totalKPH += r.KilometersPerHour
	}

	if records > 0.0 {
		avgMPS = totalMPS / records
		avgKPH = totalKPH / records
	}

	latest := FileDataPoint{
		TotalMeters:       sm.totalMetersTraveled,
		Timestamp:         time.Now().In(utc).Format(godometer.APITimeLayout),
		Meters:            float32(recordMeters),
		MetersPerSecond:   float32(avgMPS),
		KilometersPerHour: float32(avgKPH),
	}

	latestAdded := false
	dataPoints := []FileDataPoint{}
	for _, r := range sm.stats.dataPoints {
		if r.Timestamp == latest.Timestamp {
			// Same timestamp from past records, update instead of creating a new one.
			// Can easily happen if restarting quickly.
			r.Meters = r.Meters + latest.Meters
			r.MetersPerSecond = (r.MetersPerSecond + latest.MetersPerSecond) / 2
			r.KilometersPerHour = (r.KilometersPerHour + latest.KilometersPerHour) / 2
			latestAdded = true
			dataPoints = append(dataPoints, r)
		} else {
			dataPoints = append(dataPoints, r)
		}
	}

	if !latestAdded {
		dataPoints = append(dataPoints, latest)
	}

	sm.stats = NewStatsData()
	keepFrom := 0
	dataPointCount := len(dataPoints)
	if dataPointCount > keepPastDataPoints {
		keepFrom = dataPointCount - keepPastDataPoints
	}

	sm.stats.dataPoints = dataPoints[keepFrom:]
	sm.metersTraveled = 0.0
	sm.statsMutex.Unlock()

	if statsDebug {
		log.Printf("Reporting %.1fm @ %.1fm/s or %.1fkm/h", latest.Meters, latest.MetersPerSecond, latest.KilometersPerHour)
	}

	sm.writeLocalDB(dataPoints)
	sm.reportStats(dataPoints)
}

func (sm *StatsMonitor) reportStats(fdps []FileDataPoint) {
	if sm.apiBaseUrl == "" {
		// We don't want to report to anywhere
		return
	}

	var adps []godometer.UpdateDataPoint
	for _, fdp := range fdps {
		adps = append(adps, fdp.toAPIDataPoint())
	}

	payload := godometer.UpdateStatsRequest{DataPoints: adps}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal request POST data: %s. Could not report stats.", err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/updateStats", sm.apiBaseUrl)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to initialize POST request: %s. Could not report stats.", err)
		return
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", sm.apiAuth)

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		log.Printf("API error reporting stats to %s: %s", url, err)
		return
	}

	if resp.StatusCode != 200 {
		log.Printf("API returned status %d reporting stats to %s. This is likely not a good sign.", resp.StatusCode, url)
	}

	if statsDebug {
		log.Printf("Updated %d dataPoints of data to %s", len(fdps), url)
	}
}

func (sm *StatsMonitor) updateScreen() {
	log.Printf("Total meters traveled: %.1f", sm.totalMetersTraveled)
	log.Printf("Current m/s:  %.1f", sm.currentMPS)
	log.Printf("Current km/h: %.1f", sm.currentKPH)
}

func (sm *StatsMonitor) Monitor(quiet bool, exit chan bool) {
	// TODO: Save at end of each minute
	save := time.Tick(time.Minute)

	screen := make(<-chan time.Time)
	if !quiet {
		screen = time.Tick(time.Second)
	}
	for {
		select {
		case result := <-sm.results:
			// This should be synchronous
			sm.update(result)

		case <-save:
			go sm.saveStats()

		case <-screen:
			go sm.updateScreen()

		case <-exit:
			// Save before quitting
			sm.saveStats()
			return
		}
	}
}
