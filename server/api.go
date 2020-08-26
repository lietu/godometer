package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/pprof"
	ginzap "github.com/gin-contrib/zap"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
	"github.com/lietu/godometer"
)

const frontend = "../../frontend/public"

var prevFakeMeters = 0.0

const maxFakeMeters = 175.0

// YYYY-MM-DD HH:MM - we mostly want per minute precision
const (
	minuteLayout = godometer.APITimeLayout
	hourLayout   = "2006-01-02 15"
	dayLayout    = "2006-01-02"
	monthLayout  = "2006-01"
	yearLayout   = "2006"
)

// Timestamp is key, need counter for updating averages
type DBDataPoint struct {
	Counter           int64   `json:"c",firestore:"counter"`
	Meters            float32 `json:"m",firestore:"meters"`
	MetersPerSecond   float32 `json:"mps",firestore:"mps"`
	KilometersPerHour float32 `json:"kph",firestore:"kph"`
}

func (ddp *DBDataPoint) toResponseDataPoint(ts string) ResponseDataPoint {
	return ResponseDataPoint{
		Counter:           ddp.Counter,
		Timestamp:         ts,
		Meters:            ddp.Meters,
		MetersPerSecond:   ddp.MetersPerSecond,
		KilometersPerHour: ddp.KilometersPerHour,
	}
}

type ResponseDataPoint struct {
	Counter           int64   `json:"c"`
	Timestamp         string  `json:"ts"`
	Meters            float32 `json:"m"`
	MetersPerSecond   float32 `json:"mps"`
	KilometersPerHour float32 `json:"kph"`
}

type EventsResponse struct {
	Events []ResponseDataPoint `json:"events"`
}

type StatsResponse struct {
	EventTimestamps []string            `json:"eventTimestamps"`
	DataPoints      []ResponseDataPoint `json:"dataPoints"`
}

type Server struct {
	projectId  string
	lastEvents []ResponseDataPoint
	minutes    map[string]DBDataPoint
	hours      map[string]DBDataPoint
	days       map[string]DBDataPoint
	weeks      map[string]DBDataPoint
	months     map[string]DBDataPoint
	years      map[string]DBDataPoint
	engine     *gin.Engine
}

func weekFormat(ts time.Time) string {
	year, week := ts.ISOWeek()
	return fmt.Sprintf("%d week %d", year, week)
}

func (s *Server) updateStats(c *gin.Context) {
	req := &godometer.UpdateStatsRequest{}
	err := c.BindJSON(req)
	if err != nil {
		log.Printf("Failed to parse request: %s", err)
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ctx := context.Background()
	s.writeStats(ctx, req.DataPoints)
}

func getPeriodIds(period string) []string {
	if period == "years" {
		ids := Last4Years()
		return ids[:]
	} else if period == "months" {
		ids := Last12Months()
		return ids[:]
	} else if period == "weeks" {
		ids := Last5Weeks()
		return ids[:]
	} else if period == "days" {
		ids := Last7Days()
		return ids[:]
	} else if period == "hours" {
		ids := Last24Hours()
		return ids[:]
	} else if period == "minutes" {
		ids := Last60Minutes()
		return ids[:]
	}
	log.Printf("Invalid period %s", period)
	return []string{}
}

func (s *Server) returnEvents(c *gin.Context) {
	c.JSON(200, EventsResponse{
		Events: s.lastEvents,
	})
}

func (s *Server) returnRecords(period string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var availableDataPoints map[string]DBDataPoint
		if period == "years" {
			availableDataPoints = s.years
		} else if period == "months" {
			availableDataPoints = s.months
		} else if period == "weeks" {
			availableDataPoints = s.weeks
		} else if period == "days" {
			availableDataPoints = s.days
		} else if period == "hours" {
			availableDataPoints = s.hours
		} else if period == "minutes" {
			availableDataPoints = s.minutes
		} else {
			log.Printf("Invalid period %s", period)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		ids := getPeriodIds(period)

		var events []ResponseDataPoint
		for _, id := range ids {
			var event ResponseDataPoint
			adp, ok := availableDataPoints[id]
			if ok {
				event = ResponseDataPoint{
					Counter:           1,
					Timestamp:         id,
					Meters:            adp.Meters,
					MetersPerSecond:   adp.MetersPerSecond,
					KilometersPerHour: adp.KilometersPerHour,
				}
			} else {
				event = ResponseDataPoint{
					Counter:           adp.Counter,
					Timestamp:         id,
					Meters:            0.0,
					MetersPerSecond:   0.0,
					KilometersPerHour: 0.0,
				}
			}
			events = append(events, event)
		}

		var timestamps []string
		for _, e := range events {
			timestamps = append(timestamps, e.Timestamp)
		}

		response := StatsResponse{
			EventTimestamps: timestamps,
			DataPoints:      events,
		}

		c.JSON(200, response)
	}
}

func fakeDataPoint() DBDataPoint {
	metersChange := rand.Float64() * 50.0
	if prevFakeMeters-metersChange > 0 && prevFakeMeters+metersChange < maxFakeMeters {
		dir := rand.Int31n(1) == 1
		if !dir {
			metersChange = -metersChange
		}
	} else if prevFakeMeters+metersChange > maxFakeMeters {
		metersChange = -metersChange
	}

	meters := prevFakeMeters + metersChange

	mps := float32(meters / 60.0)
	kph := mps * 3600.0 / 1000.0

	prevFakeMeters = meters

	return DBDataPoint{
		Counter:           1,
		Meters:            float32(meters),
		MetersPerSecond:   mps,
		KilometersPerHour: kph,
	}
}

func (s *Server) fillFakeDataRecords(records map[string]DBDataPoint) {
	for key := range records {
		records[key] = fakeDataPoint()
	}
}

func (s *Server) generateFakeData() {
	// Initialize all data structures
	s.fillFakeDataRecords(s.years)
	s.fillFakeDataRecords(s.months)
	s.fillFakeDataRecords(s.weeks)
	s.fillFakeDataRecords(s.days)
	s.fillFakeDataRecords(s.hours)
	s.fillFakeDataRecords(s.minutes)

	log.Printf("Filled records with fake data")

	tick := time.Tick(time.Minute)
	ctx := context.Background()
	for {
		select {
		case <-tick:
			dp := fakeDataPoint()
			udp := []godometer.UpdateDataPoint{
				{
					Timestamp:         time.Now().In(utc).Format(minuteLayout),
					Meters:            dp.Meters,
					MetersPerSecond:   dp.MetersPerSecond,
					KilometersPerHour: dp.KilometersPerHour,
				},
			}

			log.Printf("FAKED: %.1f m @ %.1f m/s or %1.f km/h", udp[0].Meters, udp[0].MetersPerSecond, udp[0].KilometersPerHour)
			s.writeStats(ctx, udp)
		}
	}
}

func (s *Server) Run(listenAddr string, fakeData bool) {
	if fakeData {
		go s.generateFakeData()
	}

	err := s.engine.Run(listenAddr)
	if err != nil {
		log.Panic("Failed to run server: %s", err)
	}
}

func NewServer(dev bool, projectId string, apiAuth string) *Server {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Panicf("Failed to create logger: %s", err)
	}

	var router *gin.Engine
	if dev {
		router = gin.Default()
		pprof.Register(router)
	} else {
		gin.SetMode(gin.ReleaseMode)
		router = gin.New()
		router.Use(ginzap.Ginzap(logger, time.RFC3339, true))
		router.Use(ginzap.RecoveryWithZap(logger, true))
	}
	router.Use(SecurityMiddleware(dev))
	// It's kind of important to have gzip enabled.
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	srv := &Server{}
	srv.projectId = projectId
	srv.loadData()

	apiV1 := router.Group("/api/v1")
	apiV1.POST("/updateStats", AuthRequired(apiAuth), srv.updateStats)
	apiV1.GET("/stats/events", srv.returnEvents)
	apiV1.GET("/stats/minutes", srv.returnRecords("minutes"))
	apiV1.GET("/stats/hours", srv.returnRecords("hours"))
	apiV1.GET("/stats/days", srv.returnRecords("days"))
	apiV1.GET("/stats/weeks", srv.returnRecords("weeks"))
	apiV1.GET("/stats/months", srv.returnRecords("months"))
	apiV1.GET("/stats/years", srv.returnRecords("years"))

	files, err := ioutil.ReadDir(frontend)
	if err != nil {
		log.Panicf("Failed to read frontend files: %s", err)
	}

	for _, f := range files {
		fname := f.Name()
		src := filepath.Join(frontend, fname)
		path := fmt.Sprintf("/%s", fname)

		if fname == "index.html" {
			path = "/"
		}

		if f.IsDir() {
			router.Static(path, src)
		} else {
			router.StaticFile(path, src)
		}
	}

	srv.engine = router
	return srv
}
