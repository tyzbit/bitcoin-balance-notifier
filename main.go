package main

import (
	"embed"
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
	//go:embed web
	web embed.FS
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
		if err := watcher.DB.AutoMigrate(schemaType); err != nil {
			log.Fatal("unable to automigrate ", reflect.TypeOf(&schemaType).Elem().Name(), "err: ", err)
		}
	}

	// Set up BTC-RPC
	watcher.BTCAPI = btcapi.Config{
		ExplorerURL: watcher.BTCAPIEndpoint,
	}

	StartWatches()
	r := gin.Default()
	InitFrontend(r)
	InitBackend(r)

	if err := r.Run(":" + fmt.Sprintf(watcher.Port)); err != nil {
		log.Fatal("could not start: ", err)
	}
}
