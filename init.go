package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	cfg "github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

// InitLogging sets up logging
func (w *Watcher) InitLogging() {
	// Read from .env and override from the local environment
	dotEnvFeeder := feeder.DotEnv{Path: ".env"}
	envFeeder := feeder.Env{}

	// We need to use w.Config because we're using pointers
	_ = cfg.New().AddFeeder(dotEnvFeeder).AddStruct(&w.Config).Feed()
	_ = cfg.New().AddFeeder(envFeeder).AddStruct(&w.Config).Feed()

	// Info level by default
	LogLevelSelection := log.InfoLevel
	switch {
	case strings.EqualFold(w.LogLevel, "trace"):
		LogLevelSelection = log.TraceLevel
		log.SetReportCaller(true)
	case strings.EqualFold(w.LogLevel, "debug"):
		LogLevelSelection = log.DebugLevel
		log.SetReportCaller(true)
	case strings.EqualFold(w.LogLevel, "info"):
		LogLevelSelection = log.InfoLevel
	case strings.EqualFold(w.LogLevel, "warn"):
		LogLevelSelection = log.WarnLevel
	case strings.EqualFold(w.LogLevel, "error"):
		LogLevelSelection = log.ErrorLevel
	}
	log.SetLevel(LogLevelSelection)
	log.SetFormatter(&log.JSONFormatter{})

	if log.GetLevel() > log.DebugLevel {
		w.LogConfig = logger.Default.LogMode(logger.Info)
	}
}

func GinJSONFormatter(param gin.LogFormatterParams) string {
	jsonFormat := `{"time":"%s","clientip":"%s","method":"%s","uri":"%s","status":%3d,"latency":"%v","message":"%s","host":"%s","useragent":"%s","proto":"%s","error_msg":"%s","size":%d}` + "\n"
	return fmt.Sprintf(jsonFormat,
		param.TimeStamp.Format(TimeFormatter),
		param.ClientIP,
		param.Method,
		param.Request.RequestURI,
		param.StatusCode,
		param.Latency,
		fmt.Sprintf("%s %s %s", param.Method, param.Request.URL, param.Request.Proto),
		param.Request.Host,
		param.Request.UserAgent(),
		param.Request.Proto,
		param.ErrorMessage,
		param.BodySize,
	)
}

// FillDefaults sets a Watcher to default
// values defined in the constants
func (w *Watcher) FillDefaults() {
	// Set unitialized values to preset defaults
	if w.BTCAPIEndpoint == "" {
		w.BTCAPIEndpoint = DefaultApi
	}
	if w.LogLevel == "" {
		w.LogLevel = "info"
	}
	if w.SleepInterval == 0 {
		w.SleepInterval = DefaultSleepInterval
	}
	if w.Lookahead == 0 {
		w.Lookahead = DefaultLookahead
	}
	if w.PageSize == 0 {
		w.PageSize = DefaultPageSize
	}
	if w.DBPath == "" {
		w.DBPath = DefaultDBPath
	}
	if w.Port == "" {
		w.Port = "80"
	}
	if w.Currency == "" {
		w.Currency = CurrencyUSD
	}

	// Set up DB path
	// Create the folder path if it doesn't exist
	if _, err := os.Stat(w.DBPath); errors.Is(err, fs.ErrNotExist) {
		dirPath := filepath.Dir(w.DBPath)
		if err := os.MkdirAll(dirPath, 0660); err != nil {
			log.Warn("unable to make directory path ", dirPath, " err: ", err)
			w.DBPath = "./local.db"
		}
	}
}

// StartWatches starts goroutines for watching all of the known
// addresses in the database.
func (w *Watcher) StartWatches() {
	w.CancelWaitGroup = &sync.WaitGroup{}
	// Check balance of each address
	addresses := []AddressInfo{}
	w.DB.Model(&AddressInfo{}).Scan(&addresses)
	w.CancelSignals = map[string]chan bool{}
	for _, address := range addresses {
		// This channel is used to send a signal to stop watching the address
		cancel := make(chan bool, 1)
		w.CancelWaitGroup.Add(1)
		w.AddCancelSignal(address.Address, cancel)
		go w.WatchAddress(cancel, address.Address)
	}

	// Check balance of each key of each pubkey
	pubkeys := []PubkeyInfo{}
	w.DB.Model(&PubkeyInfo{}).Scan(&pubkeys)
	for _, pubkey := range pubkeys {
		// This channel is used to send a signal to stop watching the pubkey
		cancel := make(chan bool, 1)
		w.CancelWaitGroup.Add(1)
		w.AddCancelSignal(pubkey.Pubkey, cancel)
		go w.WatchPubkey(cancel, pubkey.Pubkey)
	}
	log.Infof("watching %d addresses and %d pubkeys", len(addresses), len(pubkeys))
}
