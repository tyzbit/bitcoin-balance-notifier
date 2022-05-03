package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	cfg "github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

const (
	TimeFormatter        string = time.RFC3339
)

// Info is an interface for AddressInfo and PubkeyInfo
type Info interface {
	GetIdentifier() string
	GetNickname() string
	Update(Watcher) error
}

// UpdateInfo calls Update() for the provided Info interface
func (w Watcher) UpdateInfo(i Info) {
	if err := i.Update(w); err != nil {
		log.Errorf("error updating address info for \"%s\" (%s): %v",
			i.GetNickname(), i.GetIdentifier(), err)
	}
}

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
	case strings.EqualFold(w.Config.LogLevel, "trace"):
		LogLevelSelection = log.TraceLevel
		log.SetReportCaller(true)
	case strings.EqualFold(w.Config.LogLevel, "debug"):
		LogLevelSelection = log.DebugLevel
		log.SetReportCaller(true)
	case strings.EqualFold(w.Config.LogLevel, "info"):
		LogLevelSelection = log.InfoLevel
	case strings.EqualFold(w.Config.LogLevel, "warn"):
		LogLevelSelection = log.WarnLevel
	case strings.EqualFold(w.Config.LogLevel, "error"):
		LogLevelSelection = log.ErrorLevel
	}
	log.SetLevel(LogLevelSelection)
	log.SetFormatter(&log.JSONFormatter{})

	if log.GetLevel() > log.DebugLevel {
		w.LogConfig = logger.Default.LogMode(logger.Info)
	}
}

func GinJSONFormatter(param gin.LogFormatterParams) string {
	jsonFormat := `{"level":"%s","time":"%s","clientip":"%s","method":"%s","uri":"%s","status":%3d,"latency":"%v","message":"%s","host":"%s","useragent":"%s","proto","%s","error_msg":"%s","size":%d}` + "\n"
	return fmt.Sprintf(jsonFormat,
		watcher.Config.LogLevel,
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
	if w.Config.BTCAPIEndpoint == "" {
		w.Config.BTCAPIEndpoint = DefaultApi
	}
	if w.Config.SleepInterval == 0 {
		w.Config.SleepInterval = DefaultSleepInterval
	}
	if w.Config.Lookahead == 0 {
		w.Config.Lookahead = DefaultLookahead
	}
	if w.Config.PageSize == 0 {
		w.Config.PageSize = DefaultPageSize
	}
	if w.Config.DBPath == "" {
		w.Config.DBPath = DefaultDBPath
	}
	if w.Config.Port == "" {
		w.Config.Port = "80"
	}
	if w.Config.Currency == "" {
		w.Config.Currency = CurrencyUSD
	}

	// Set up DB path
	// Create the folder path if it doesn't exist
	if _, err := os.Stat(w.Config.DBPath); errors.Is(err, fs.ErrNotExist) {
		dirPath := filepath.Dir(w.Config.DBPath)
		if err := os.MkdirAll(dirPath, 0660); err != nil {
			log.Warn("unable to make directory path ", dirPath, " err: ", err)
			w.Config.DBPath = "./local.db"
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

// AddCancelSignal adds a cancel channel to w.CancelSignals
func (w Watcher) AddCancelSignal(i string, c chan bool) {
	w.CancelSignals[i] = c
	w.CancelWaitGroup.Done()
}

// DeleteCancelSignal deletes the cancel channel on w.CancelSignals
func (w Watcher) DeleteCancelSignal(i string) {
	// Only send to the channel if it's open and buffered
	if w.CancelSignals[i] != make(chan bool, 1) {
		w.CancelSignals[i] <- true
	}
	delete(w.CancelSignals, i)
	w.CancelWaitGroup.Done()
}

// SendNotification sends a message filled with values from a template
// to Discord
func (w Watcher) SendNotification(i interface{}, mt string) {
	t, err := template.New("addressMessage").Parse(mt)
	if err != nil {
		log.Error("error setting up template: ", err)
		return
	}

	var b bytes.Buffer
	err = t.Execute(&b, i)
	if err != nil {
		log.Errorf("error executing template, err: %v", err)
		return
	}

	message := b.String()
	payload := DiscordPayload{
		Content: message,
	}
	var m bytes.Buffer
	err = json.NewEncoder(&m).Encode(payload)
	if err != nil {
		log.Errorf("unable to encode payload: %+v", payload)
		return
	}

	client := http.Client{}
	resp, respErr := client.Post(w.Config.DiscordWebhook, "application/json", &m)
	if respErr != nil || resp.StatusCode != 204 {
		log.Errorf("error calling Discord API (%s): %v", resp.Status, respErr)
		return
	}
}
