package main

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/tyzbit/btcapi"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Watcher struct {
	BTCAPI              btcapi.Config
	CancelWaitGroup     *sync.WaitGroup
	CancelSignals       map[string]chan bool
	DB                  *gorm.DB
	LogConfig           logger.Interface
	BTCAPIEndpoint      string `env:"BTC_RPC_API"`
	CheckAllPubkeyTypes bool   `env:"CHECK_ALL_PUBKEY_TYPES"`
	Currency            string `env:"CURRENCY"`
	DBPath              string `env:"DB_PATH"`
	DiscordWebhook      string `env:"DISCORD_WEBHOOK"`
	SleepInterval       int    `env:"SLEEP_INTERVAL"`
	LogLevel            string `env:"LOG_LEVEL"`
	Lookahead           int    `env:"LOOKAHEAD"`
	PageSize            int    `env:"PAGE_SIZE"`
	Port                string `env:"PORT"`
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

	watcher.CancelWaitGroup = &sync.WaitGroup{}
	// Check balance of each address
	addresses := []AddressInfo{}
	watcher.DB.Model(&AddressInfo{}).Scan(&addresses)
	watcher.CancelSignals = map[string]chan bool{}
	for _, address := range addresses {
		cancel := make(chan bool, 1)
		watcher.CancelWaitGroup.Add(1)
		watcher.AddCancelSignal(address.Address, cancel)
		go watcher.WatchAddress(cancel, address.Address)
	}

	// Check balance of each key of each pubkey
	pubkeys := []PubkeyInfo{}
	watcher.DB.Model(&PubkeyInfo{}).Scan(&pubkeys)
	for _, pubkey := range pubkeys {
		cancel := make(chan bool, 1)
		watcher.CancelWaitGroup.Add(1)
		watcher.AddCancelSignal(pubkey.Pubkey, cancel)
		go watcher.WatchPubkey(cancel, pubkey.Pubkey)
	}

	log.Infof("watching %d addresses and %d pubkeys", len(addresses), len(pubkeys))

	r := gin.Default()

	// Backend
	r.POST("/balance", watcher.GetBalance)
	r.POST("/watch", watcher.AddWatch)
	r.GET("/balances", watcher.GetBalances)
	r.GET("/watches", watcher.GetWatches)
	r.DELETE("/identifier", watcher.DeleteIdentifier)

	// Frontend
	r.LoadHTMLGlob("web/templates/**/*")
	r.Static("/static", "web/static")
	r.GET("/", watcher.Home)

	thing := ":" + fmt.Sprintf(watcher.Port)
	err = r.Run(thing)
	if err != nil {
		log.Fatal("could not start: ", err)
	}
}
