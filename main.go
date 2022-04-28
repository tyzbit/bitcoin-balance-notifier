package main

import (
	"os"
	"os/signal"
	"reflect"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/tyzbit/btcapi"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Watcher struct {
	Addresses           []string `env:"ADDRESSES"`
	BTCAPIEndpoint      string   `env:"BTC_RPC_API"`
	BTCAPI              btcapi.Config
	CheckAllPubkeyTypes bool   `env:"CHECK_ALL_PUBKEY_TYPES"`
	Currency            string `env:"CURRENCY"`
	DB                  *gorm.DB
	DBPath              string `env:"DB_PATH"`
	DiscordWebhook      string `env:"DISCORD_WEBHOOK"`
	SleepInterval       int    `env:"SLEEP_INTERVAL"`
	LogConfig           logger.Interface
	LogLevel            string   `env:"LOG_LEVEL"`
	Lookahead           int      `env:"LOOKAHEAD"`
	PageSize            int      `env:"PAGE_SIZE"`
	Pubkeys             []string `env:"PUBKEYS"`
}

type DiscordPayload struct {
	Content string `json:"content"`
}

const (
	DefaultApi           string = "https://bitcoinexplorer.org"
	DefaultDBPath        string = "/db/addresses.sqlite"
	DefaultLookahead     int    = 20
	DefaultPageSize      int    = 100
	DefaultSleepInterval int    = 300
	SatsPerBitcoin       int    = 100000000
)

var (
	allSchemaTypes = []interface{}{
		&AddressInfo{},
		&PubkeyInfo{},
	}
	watcher Watcher
)

func init() {
	InitLogging()

	// Set defaults and throw errors if necessary values aren't set
	watcher.FillDefaults()
}

func main() {
	db, err := gorm.Open(sqlite.Open(watcher.DBPath), &gorm.Config{Logger: watcher.LogConfig})
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
	watcher.BTCAPI = btcapi.Config{
		ExplorerURL: watcher.BTCAPIEndpoint,
	}

	// Check balance of each address
	for _, address := range watcher.Addresses {
		go watcher.WatchAddress(address)
	}

	// Check balance of each key of each pubkey
	for _, pubkey := range watcher.Pubkeys {
		go watcher.WatchPubkey(pubkey)
	}

	log.Infof("watching %d addresses and %d pubkeys", len(watcher.Addresses), len(watcher.Pubkeys))

	// Listen for signals from the OS
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
