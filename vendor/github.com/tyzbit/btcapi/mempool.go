package btcapi

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const MempoolRoute string = "/mempool"

type Fees struct {
	NextBlock     int `json:"nextBlock"`
	ThirtyMinutes int `json:"30min"`
	SixtyMinutes  int `json:"60min"`
	OneDay        int `json:"1day"`
}

// MempoolCount returns the number of transactions in the mempool.
func (c Config) MempoolCount() (count int, err error) {
	url := c.ExplorerURL + api + MempoolRoute + "/count"
	body, err := getAPI(url)
	if err != nil {
		return count, err
	}

	count, err = strconv.Atoi(string(body))
	if err != nil {
		return count, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return count, nil
}

// Fees returns the recommended fees to get included within pre-set deadlines.
func (c Config) Fees() (fees Fees, err error) {
	url := c.ExplorerURL + api + MempoolRoute + "/fees"
	body, err := getAPI(url)
	if err != nil {
		return fees, err
	}

	err = json.Unmarshal(body, &fees)
	if err != nil {
		return fees, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return fees, nil
}
