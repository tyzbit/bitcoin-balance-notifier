package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	cfg "github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/logger"
)

// Info is an interface for AddressInfo and PubkeyInfo
type Info interface {
	GetIdentifier() string
	GetNickname() string
	Update(Watcher) error
}

// UpdateInfo calls Update() for the provided Info interface
func (w Watcher) UpdateInfo(i Info) {
	err := i.Update(w)
	if err != nil {
		log.Errorf("error updating address info for \"%s\" (%s): %v",
			i.GetNickname(), i.GetIdentifier(), err)
	}
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

// InitLogging sets up logging
func InitLogging() {
	// Read from .env and override from the local environment
	dotEnvFeeder := feeder.DotEnv{Path: ".env"}
	envFeeder := feeder.Env{}

	_ = cfg.New().AddFeeder(dotEnvFeeder).AddStruct(&watcher).Feed()
	_ = cfg.New().AddFeeder(envFeeder).AddStruct(&watcher).Feed()

	// Info level by default
	LogLevelSelection := log.InfoLevel
	switch {
	case strings.EqualFold(watcher.LogLevel, "trace"):
		LogLevelSelection = log.TraceLevel
		log.SetReportCaller(true)
	case strings.EqualFold(watcher.LogLevel, "debug"):
		LogLevelSelection = log.DebugLevel
		log.SetReportCaller(true)
	case strings.EqualFold(watcher.LogLevel, "info"):
		LogLevelSelection = log.InfoLevel
	case strings.EqualFold(watcher.LogLevel, "warn"):
		LogLevelSelection = log.WarnLevel
	case strings.EqualFold(watcher.LogLevel, "error"):
		LogLevelSelection = log.ErrorLevel
	}
	log.SetLevel(LogLevelSelection)
	log.SetFormatter(&log.JSONFormatter{})

	if log.GetLevel() > log.DebugLevel {
		watcher.LogConfig = logger.Default.LogMode(logger.Info)
	}
}

// FillDefaults sets a Watcher to default
// values defined in the constants
func (w Watcher) FillDefaults() {
	// Set unitialized values to preset defaults
	if w.BTCAPIEndpoint == "" {
		watcher.BTCAPIEndpoint = DefaultApi
	}
	if w.SleepInterval == 0 {
		watcher.SleepInterval = DefaultSleepInterval
	}
	if w.Lookahead == 0 {
		watcher.Lookahead = DefaultLookahead
	}
	if w.PageSize == 0 {
		watcher.PageSize = DefaultPageSize
	}
	if w.DBPath == "" {
		watcher.DBPath = DefaultDBPath
	}

	// Set up DB path
	// Create the folder path if it doesn't exist
	_, err := os.Stat(watcher.DBPath)
	if errors.Is(err, fs.ErrNotExist) {
		dirPath := filepath.Dir(watcher.DBPath)
		if err := os.MkdirAll(dirPath, 0660); err != nil {
			log.Warn("unable to make directory path ", dirPath, " err: ", err)
			watcher.DBPath = "./local.db"
		}
	}
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
	resp, respErr := client.Post(w.DiscordWebhook, "application/json", &m)
	if respErr != nil || resp.StatusCode != 204 {
		log.Errorf("error calling Discord API (%s): %v", resp.Status, respErr)
		return
	}
}
