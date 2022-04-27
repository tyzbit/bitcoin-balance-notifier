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
	"strconv"
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
	Addresses           []string `env:"ADDRESSES"`
	BTCRPCAPI           string   `env:"BTC_RPC_API"`
	CheckAllPubkeyTypes bool     `env:"CHECK_ALL_PUBKEY_TYPES"`
	Currency            string   `env:"CURRENCY"`
	DB                  *gorm.DB
	DiscordWebhook      string   `env:"DISCORD_WEBHOOK"`
	SleepInterval       int      `env:"SLEEP_INTERVAL"`
	LogLevel            string   `env:"LOG_LEVEL"`
	Lookahead           int      `env:"LOOKAHEAD"`
	PageSize            int      `env:"PAGE_SIZE"`
	Pubkeys             []string `env:"PUBKEYS"`
}

type AddressInfo struct {
	Address             string
	Nickname            string
	BalanceSat          int
	PreviousBalanceSat  int
	BalanceFiat         string
	PreviousBalanceFiat string
	Currency            string
	TXCount             int
}

type PubkeyInfo struct {
	Pubkey              string
	Nickname            string
	BalanceSat          int
	PreviousBalanceSat  int
	BalanceFiat         string
	PreviousBalanceFiat string
	Currency            string
	TXCount             int
}

type DiscordPayload struct {
	Content string `json:"content"`
}

const (
	DefaultApi     string = "https://bitcoinexplorer.org"
	SatsPerBitcoin int    = 100000000
)

var (
	allSchemaTypes = []interface{}{
		&AddressInfo{},
		&PubkeyInfo{},
	}
	addressMessageTemplate = `**Address Balance Changed**
Nickname: {{ .Nickname }}
Address: {{ .Address }}
Previous Balance (satoshis): {{ .PreviousBalanceSat }}
Previous Balance ({{ .Currency }}): {{ .PreviousBalanceFiat }}
Transactions: {{ .TXCount }}
New Balance (satoshis): {{ .BalanceSat }}
New Balance ({{ .Currency }}): {{ .BalanceFiat }}
`
	pubkeyMessageTemplate = `**Pubkey Balance Changed**
Nickname: {{ .Nickname }}
Address: {{ .Pubkey }}
Previous Balance (satoshis): {{ .PreviousBalanceSat }}
Previous Balance ({{ .Currency }}): {{ .PreviousBalanceFiat }}
Transactions: {{ .TXCount }}
New Balance (satoshis): {{ .BalanceSat }}
New Balance ({{ .Currency }}): {{ .BalanceFiat }}
`
	sqlitePath string = "/db/addresses.sqlite"
	watcher    WatcherConfig
)

// WatchAddress takes a btcapi config and a nickname:address string. It
// checks the database for a previous address summary and compares the
// previous balance to the current balance. If they are different,
// it calls WatcherConfig.NotifyHandler
func (w WatcherConfig) WatchAddress(c btcapi.Config, nickAddress string) {
	for {
		nickname := strings.Split(nickAddress, ":")[0]
		address := strings.Split(nickAddress, ":")[1]

		var oldAddressInfo AddressInfo
		w.DB.Model(&AddressInfo{}).
			Where(&AddressInfo{Address: address, Nickname: nickname}).
			Scan(&oldAddressInfo)

		// Insert blank AddressInfo if none was found
		if (oldAddressInfo == AddressInfo{}) {
			log.Warnf("previous address information for \"%s\" (%s) was not found, database will be updated", nickname, address)
			oldAddressInfo = AddressInfo{
				Address:             address,
				Nickname:            nickname,
				BalanceSat:          0,
				BalanceFiat:         "0.0",
				Currency:            w.Currency,
				PreviousBalanceSat:  0,
				PreviousBalanceFiat: "0.0",
				TXCount:             0,
			}
			tx := w.DB.Model(&AddressInfo{}).Create(&oldAddressInfo)
			if tx.RowsAffected != 1 {
				log.Errorf("%d rows affected creating address info for \"%s\" (%s)", tx.RowsAffected, nickname, address)
			}
		}

		addressSummary, err := c.AddressSummary(address)
		if err != nil {
			log.Errorf("error calling btcapi: %v", err)
		}

		price, err := c.Price()
		if err != nil {
			log.Errorf("error calling btcapi: %v", err)
		}

		balanceFiat := 0.0
		currency := strings.ToUpper(w.Currency)
		bitcoinBalance := float64(addressSummary.TXHistory.BalanceSat) / float64(SatsPerBitcoin)
		switch currency {
		case "EUR":
			balanceFiat = price.EUR * bitcoinBalance
		case "GBP":
			balanceFiat = price.GBP * bitcoinBalance
		case "XAU":
			balanceFiat = price.XAU * bitcoinBalance
		default:
			currency = "USD"
			balanceFiat = price.USD * bitcoinBalance
		}
		addressInfo := AddressInfo{
			Address:             address,
			Nickname:            nickname,
			BalanceSat:          addressSummary.TXHistory.BalanceSat,
			BalanceFiat:         strconv.FormatFloat(balanceFiat, 'f', 2, 64),
			Currency:            currency,
			PreviousBalanceSat:  oldAddressInfo.BalanceSat,
			PreviousBalanceFiat: oldAddressInfo.BalanceFiat,
			TXCount:             addressSummary.TXHistory.TXCount,
		}

		if addressSummary.TXHistory.BalanceSat != oldAddressInfo.BalanceSat {
			log.Debug(address, " balance updated from ", oldAddressInfo.BalanceSat, " to ", addressSummary.TXHistory.BalanceSat)
			tx := w.DB.Model(&AddressInfo{}).
				Where(&AddressInfo{Address: address, Nickname: nickname}).
				Updates(&addressInfo)
			if tx.RowsAffected != 1 {
				log.Errorf("%d rows affected updating balance for \"%s\" (%s)", tx.RowsAffected, nickname, address)
			}
			w.AddressNotifyHandler(addressInfo)
		}
		time.Sleep(time.Duration(int(time.Second) * w.SleepInterval))
	}
}

// WatchPubkey takes a btcapi config and a nickname:pubkey string. It
// checks the database for a previous pubkey summary and compares the
// previous balance to the current balance. If they are different,
// it calls WatcherConfig.NotifyHandler
func (w WatcherConfig) WatchPubkey(c btcapi.Config, nickPubkey string) {
	for {
		nickname := strings.Split(nickPubkey, ":")[0]
		var pubKeys []string
		pubKeys = append(pubKeys, strings.Split(nickPubkey, ":")[1])

		var oldPubkeyInfo PubkeyInfo
		w.DB.Model(&PubkeyInfo{}).
			Where(&PubkeyInfo{Pubkey: pubKeys[0], Nickname: nickname}).
			Scan(&oldPubkeyInfo)

		// Insert a blank PubkeyInfo if none was found
		if (oldPubkeyInfo == PubkeyInfo{}) {
			log.Warnf("previous address information for \"%s\" (%s) was not found, database will be updated", nickname, pubKeys[0])
			oldPubkeyInfo = PubkeyInfo{
				Pubkey:              pubKeys[0],
				Nickname:            nickname,
				BalanceSat:          0,
				BalanceFiat:         "0.0",
				Currency:            w.Currency,
				PreviousBalanceSat:  0,
				PreviousBalanceFiat: "0.0",
				TXCount:             0,
			}
			tx := w.DB.Model(&PubkeyInfo{}).Create(&oldPubkeyInfo)
			if tx.RowsAffected != 1 {
				log.Errorf("%d rows affected creating pubkey info for \"%s\" (%s)", tx.RowsAffected, nickname, pubKeys[0])
			}
		}

		pubkeySummary, err := c.ExtendedPublicKeyDetails(pubKeys[0])
		if err != nil {
			log.Errorf("error calling btcapi: %v", err)
			continue
		}

		if w.CheckAllPubkeyTypes {
			for _, pubkeyType := range pubkeySummary.RelatedKeys {
				pubKeys = append(pubKeys, pubkeyType.Key)
			}
		}
		// totalBalance is the balance of all pubkeys, similar for totalTxCount
		totalBalance, totalTxCount := 0, 0
		for _, pubkey := range pubKeys {
			// totalPubkeyBalance is the balance for this pubkey, similar
			// for totalPubkeyTxCount. NoTXCount is incremented when
			// a consecutive address check yields no new transactions
			totalPubkeyBalance, totalPubkeyTxCount, NoTXCount := 0, 0, 0
		pubkey:
			for offset := 0; 0 == 0; offset = offset + w.PageSize {
				pubKeyPage, err := c.ExtendedPublicKeyDetailsPage(pubkey, w.PageSize, offset)
				if err != nil {
					log.Errorf("error calling btcapi: %v", err)
				}
				pubkeyTxCount := 0
				for _, address := range pubKeyPage.ReceiveAddresses {
					addressSummary, err := c.AddressSummary(address)
					if err != nil {
						log.Errorf("error calling btcapi: %v", err)
						continue
					}
					totalPubkeyBalance = totalPubkeyBalance + addressSummary.TXHistory.BalanceSat
					totalPubkeyTxCount = totalPubkeyTxCount + addressSummary.TXHistory.TXCount
					if pubkeyTxCount == addressSummary.TXHistory.TXCount {
						if NoTXCount > w.Lookahead {
							break pubkey
						}
						NoTXCount++
					}
					pubkeyTxCount = addressSummary.TXHistory.TXCount
				}
			}
			totalBalance = totalBalance + totalPubkeyBalance
			totalTxCount = totalTxCount + totalPubkeyTxCount
		}
		price, err := c.Price()
		if err != nil {
			log.Errorf("error calling btcapi: %v", err)
		}

		balanceFiat := 0.0
		currency := strings.ToUpper(w.Currency)
		bitcoinBalance := float64(totalBalance) / float64(SatsPerBitcoin)
		switch currency {
		case "EUR":
			balanceFiat = price.EUR * bitcoinBalance
		case "GBP":
			balanceFiat = price.GBP * bitcoinBalance
		case "XAU":
			balanceFiat = price.XAU * bitcoinBalance
		default:
			currency = "USD"
			balanceFiat = price.USD * bitcoinBalance
		}
		pubkeyInfo := PubkeyInfo{
			Pubkey:              pubKeys[0],
			Nickname:            nickname,
			BalanceSat:          totalBalance,
			PreviousBalanceSat:  oldPubkeyInfo.BalanceSat,
			BalanceFiat:         strconv.FormatFloat(balanceFiat, 'f', 2, 64),
			Currency:            currency,
			PreviousBalanceFiat: oldPubkeyInfo.BalanceFiat,
			TXCount:             totalTxCount,
		}
		if pubkeyInfo.BalanceSat != oldPubkeyInfo.BalanceSat {
			log.Debug(pubKeys[0], " balance updated from ", oldPubkeyInfo.BalanceSat, " to ", pubkeyInfo.BalanceSat)
			tx := w.DB.Model(&PubkeyInfo{}).
				Where(&PubkeyInfo{Pubkey: pubKeys[0], Nickname: nickname}).
				Updates(&pubkeyInfo)
			if tx.RowsAffected != 1 {
				log.Errorf("%d rows affected updating balance for \"%s\" (%s)", tx.RowsAffected, nickname, pubKeys[0])
			}
			w.PubkeyNotifyHandler(pubkeyInfo)
		}
		time.Sleep(time.Duration(int(time.Second) * w.SleepInterval))
	}
}

// AddressNotifyHandler takes an AddressInfo as an argument and calls a Discord webhook
// with a templated message as the content
func (w WatcherConfig) AddressNotifyHandler(a AddressInfo) {
	t, err := template.New("addressMessage").Parse(addressMessageTemplate)
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

// NotifyHandler takes an AddressInfo as an argument and calls a Discord webhook
// with a templated message as the content
func (w WatcherConfig) PubkeyNotifyHandler(p PubkeyInfo) {
	t, err := template.New("pubkeyMessage").Parse(pubkeyMessageTemplate)
	if err != nil {
		log.Error("error setting up template: ", err)
		return
	}

	var b bytes.Buffer
	err = t.Execute(&b, p)
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
	if watcher.SleepInterval == 0 {
		watcher.SleepInterval = 300
	}
	if watcher.Lookahead == 0 {
		watcher.Lookahead = 20
	}
	if watcher.PageSize == 0 {
		watcher.PageSize = 100
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
		ExplorerURL: watcher.BTCRPCAPI,
	}

	// Check balance of each address
	for _, address := range watcher.Addresses {
		go watcher.WatchAddress(btcapi, address)
	}

	// Check balance of each key of each pubkey
	for _, pubkey := range watcher.Pubkeys {
		go watcher.WatchPubkey(btcapi, pubkey)
	}

	log.Infof("watching %d addresses and %d pubkeys", len(watcher.Addresses), len(watcher.Pubkeys))

	// Listen for signals from the OS
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
