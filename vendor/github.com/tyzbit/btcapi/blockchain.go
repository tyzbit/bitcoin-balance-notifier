package btcapi

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type UTXSOSet struct {
	Height              int    `json:"height"`
	BestBlock           string `json:"bestblock"`
	TXOuts              int    `json:"txouts"`
	BogoSize            int    `json:"bogosize"`
	HashSerialized2     string `json:"hash_serialized_2"`
	TotalAmount         string `json:"total_amount"`
	Transactions        int    `json:"transactions"`
	DiskSize            int    `json:"disk_size"`
	UsingCoinStatsIndex bool   `json:"usingCoinStatsIndex"`
	LastUpdated         int    `json:"lastUpdated"`
}

const BlockchainRoute string = "/blockchain"

// Coins returns the current supply of Bitcoin.
func (c Config) Coins() (coins float64, err error) {
	url := c.ExplorerURL + api + BlockchainRoute + "/coins"
	body, err := getAPI(url)
	if err != nil {
		return 0, err
	}

	coins, err = strconv.ParseFloat(string(body), 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return coins, nil
}

// UTXOSet returns the current UTXO snapshot.
func (c Config) UTXOSet() (set UTXSOSet, err error) {
	url := c.ExplorerURL + api + BlockchainRoute + "/utxo-set"
	body, err := getAPI(url)
	if err != nil {
		return set, err
	}

	err = json.Unmarshal(body, &set)
	if err != nil {
		return set, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return set, nil
}
