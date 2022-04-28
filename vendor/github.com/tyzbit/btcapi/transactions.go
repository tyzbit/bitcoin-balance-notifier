package btcapi

import (
	"encoding/json"
	"fmt"
)

const TxRoute string = "/tx/"

type TXSummary struct {
	TXID     string `json:"txid"`
	Hash     string `json:"hash"`
	Version  int    `json:"version"`
	Size     int    `json:"size"`
	VSize    int    `json:"vsize"`
	Weight   int    `json:"Weight"`
	Locktime int    `json:"locktime"`
	VIn      []struct {
		TXID      string `json:"txid"`
		VOut      int    `json:"vout"`
		ScriptSig struct {
			ASM      string `json:"asm"`
			Hex      string `json:"hex"`
			Sequence int    `json:"sequence"`
		} `json:"scriptSig"`
	} `json:"vin"`
	VOut []struct {
		Value     int `json:"value"`
		N         int `json:"n"`
		ScriptSig struct {
			ASM  string `json:"asm"`
			Hex  string `json:"hex"`
			Type string `json:"type"`
		} `json:"scriptSig"`
	} `json:"vout"`
	Hex           string `json:"hex"`
	BlockHash     string `json:"blockhash"`
	Confirmations int    `json:"confirmations"`
	BlockTime     int    `json:"blocktime"`
}

// TXSummary takes a Bitcoin transaction ID and returns an TXSummary object.
func (c Config) Tx(tx string) (TXSummary, error) {
	var TXSummary TXSummary
	body, err := getAPI(c.ExplorerURL + api + TxRoute + tx)
	if err != nil {
		return TXSummary, err
	}

	err = json.Unmarshal(body, &TXSummary)
	if err != nil {
		return TXSummary, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return TXSummary, nil
}
