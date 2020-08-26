package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	stackdriver "github.com/tommy351/zap-stackdriver"
	"go.uber.org/zap/zapcore"

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

var logger = getLogger()

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

func getLogger() *zap.Logger {
	config := &zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		Encoding:         "json",
		EncoderConfig:    stackdriver.EncoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	zapLogger, err := config.Build(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return &stackdriver.Core{
			Core: core,
		}
	}), zap.Fields(
		stackdriver.LogServiceContext(&stackdriver.ServiceContext{
			Service: "foo",
			Version: "bar",
		}),
	))
	if err != nil {
		log.Panicf("Failed to create logger: %s", err)
	}

	return zapLogger
}

func weekFormat(ts time.Time) string {
	year, week := ts.ISOWeek()
	return fmt.Sprintf("%d week %d", year, week)
}

func (s *Server) updateStats(c *gin.Context) {
	req := &godometer.UpdateStatsRequest{}
	err := c.BindJSON(req)
	if err != nil {
		logger.Warn("Failed to parse request", zap.Error(err))
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
	logger.Warn("Invalid period", zap.String("period", period))
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
			logger.Warn("Invalid period", zap.String("period", period))
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
	var router *gin.Engine
	if dev {
		router = gin.Default()
		router.Use(ginzap.Ginzap(logger, time.RFC3339, true))
		router.Use(ginzap.RecoveryWithZap(logger, true))
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
