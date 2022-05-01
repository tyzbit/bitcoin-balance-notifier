package main

import (
	"fmt"
	"strings"
)

// ConvertBalance takes a balance in satoshis and returns the equivalent
// balance in the currency specified by Watcher.Currency
func (w Watcher) ConvertBalance(balanceSat int) (float64, error) {
	price, err := w.BTCAPI.Price()
	if err != nil {
		return 0.0, fmt.Errorf("error calling btcapi: %v", err)
	}

	balanceFiat := 0.0
	currency := strings.ToUpper(w.Currency)
	bitcoinBalance := float64(balanceSat) / float64(SatsPerBitcoin)
	switch currency {
	case "USD":
		balanceFiat = price.USD * bitcoinBalance
	case "EUR":
		balanceFiat = price.EUR * bitcoinBalance
	case "GBP":
		balanceFiat = price.GBP * bitcoinBalance
	case "XAU":
		balanceFiat = price.XAU * bitcoinBalance
	default:
		balanceFiat = price.USD * bitcoinBalance
	}
	return balanceFiat, nil
}
