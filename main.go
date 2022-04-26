package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"text/template"
	"time"

	cfg "github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	log "github.com/sirupsen/logrus"
	"github.com/tyzbit/btcapi"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type WatcherConfig struct {
	Addresses      []string `env:"ADDRESSES"`
	BTCRPCAPI      string   `env:"BTC_RPC_API"`
	DB             *gorm.DB
	DiscordWebhook string `env:"DISCORD_WEBHOOK"`
	Interval       int    `env:"INTERVAL"`
	LogLevel       string `env:"LOG_LEVEL"`
}

type AddressInfo struct {
	// TODO: Add address nicknames
	Address            string
	BalanceSat         int
	PreviousBalanceSat int
	TXCount            int
}

type DiscordPayload struct {
	Content string `json:"content"`
}

const DefaultApi string = "https://bitcoinexplorer.org/"

var (
	allSchemaTypes = []interface{}{
		&AddressInfo{},
	}
	messageTemplate = `**Address Balance Changed**
Address: {{ .Address }}
Previous Balance: {{ .PreviousBalanceSat }}
New Balance: {{ .BalanceSat }}
`
	sqlitePath string = "/db/addresses.sqlite"
	watcher    WatcherConfig
)

// WatchAddress takes a btcapi config and an address. It checks the database
// for a previous address summary and compares the previous balance to the
// current balance. If they are different, it calls WatcherConfig.NotifyHandler
func (w WatcherConfig) WatchAddress(c btcapi.Config, address string) {
	for {
		var oldAddressInfo AddressInfo
		w.DB.Model(&AddressInfo{}).
			Where(&AddressInfo{Address: address}).
			Scan(&oldAddressInfo)
		if (oldAddressInfo == AddressInfo{}) {
			log.Warn("previous address information for address ", address, " not found, database will be updated")
			oldAddressInfo = AddressInfo{
				Address:            address,
				BalanceSat:         0,
				PreviousBalanceSat: 0,
				TXCount:            0,
			}
			w.DB.Model(&AddressInfo{}).Create(&oldAddressInfo)
		}
		addressSummary, err := c.AddressSummary(address)
		if err != nil {
			log.Errorf("error calling btc-rpc: %v", err)

		}
		addressInfo := AddressInfo{
			Address:            address,
			BalanceSat:         addressSummary.TXHistory.BalanceSat,
			PreviousBalanceSat: oldAddressInfo.BalanceSat,
			TXCount:            addressSummary.TXHistory.TXCount,
		}
		if addressSummary.TXHistory.BalanceSat != oldAddressInfo.BalanceSat {
			log.Debug(address, " balance updated from ", oldAddressInfo.BalanceSat, " to ", addressSummary.TXHistory.BalanceSat)
			tx := w.DB.Model(&AddressInfo{}).
				Where(&AddressInfo{Address: address}).
				Updates(&addressInfo)
			if tx.RowsAffected != 1 {
				log.Error(tx.RowsAffected, " rows affected updating balance for ", address)
			}
			w.NotifyHandler(addressInfo)
		}
		time.Sleep(time.Duration(int(time.Second) * w.Interval))
	}
}

// NotifyHandler takes an AddressInfo as an argument and calls a Discord webhook
// with a templated message as the content
func (w WatcherConfig) NotifyHandler(a AddressInfo) {
	t, err := template.New("message").Parse(messageTemplate)
	if err != nil {
		log.Error("error setting up template: ", err)
		return
	}

	var b bytes.Buffer
	err = t.Execute(&b, a)
	if err != nil {
		log.Error("error executing template, err: %w", err)
	}
	message := b.String()
	payload := DiscordPayload{
		Content: message,
	}
	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(payload)
	if err != nil {
		log.Error("unable to encode payload: ", payload)
		return
	}
	client := http.Client{}
	log.Debug("calling Discord webhook with message: ", message)
	resp, respErr := client.Post(w.DiscordWebhook, "application/json", &buf)
	if respErr != nil || resp.StatusCode != 204 {
		log.Warn("error calling Discord API (", resp.Status, "): ", respErr)
	}
}

func init() {
	// Read from .env and override from the local environment
	dotEnvFeeder := feeder.DotEnv{Path: ".env"}
	envFeeder := feeder.Env{}

	_ = cfg.New().AddFeeder(dotEnvFeeder).AddStruct(&watcher).Feed()
	_ = cfg.New().AddFeeder(envFeeder).AddStruct(&watcher).Feed()

	// Set defaults and throw errors if necessary values aren't set
	if watcher.BTCRPCAPI == "" {
		watcher.BTCRPCAPI = DefaultApi
	}
	if watcher.Interval == 0 {
		watcher.Interval = 300
	}
	if watcher.Addresses == nil || watcher.DiscordWebhook == "" {
		log.Fatal("You must provide at least one address to watch (ADDRESSES env var) and a Discord Webhook (DISCORD_WEBHOOK env var)")
	}

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
}

func main() {
	// Increase verbosity of the database if the loglevel is higher than Info
	var logConfig logger.Interface
	if log.GetLevel() > log.DebugLevel {
		logConfig = logger.Default.LogMode(logger.Info)
	}

	// Set up DB
	// Create the folder path if it doesn't exist
	_, err := os.Stat(sqlitePath)
	if errors.Is(err, fs.ErrNotExist) {
		dirPath := filepath.Dir(sqlitePath)
		if err := os.MkdirAll(dirPath, 0660); err != nil {
			log.Warn("unable to make directory path ", dirPath, " err: ", err)
			sqlitePath = "./local.db"
		}
	}
	db, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{Logger: logConfig})
	if err != nil {
		log.Fatal("unable to open db: ", err)
	}
	watcher.DB = db

	for _, schemaType := range allSchemaTypes {
		err := watcher.DB.AutoMigrate(schemaType)
		if err != nil {
			log.Fatal("unable to automigrate ", reflect.TypeOf(&schemaType).Elem().Name(), "err: ", err)
		}
	}

	// Set up BTC-RPC
	btcapi := btcapi.Config{
		APIEndpoint: watcher.BTCRPCAPI,
	}

	// Check balance of each address
	for _, address := range watcher.Addresses {
		go watcher.WatchAddress(btcapi, address)
	}

	log.Info("watching ", len(watcher.Addresses), " addresses")

	// Listen for signals from the OS
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
