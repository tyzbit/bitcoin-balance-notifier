package main

import (
	"fmt"
	"math"
)

const (
	CurrencyUSD = "USD"
	CurrencyEUR = "EUR"
	CurrencyGBP = "GBP"
	CurrencyXAU = "XAU"
)

// ConvertBalance takes a currency and one or more balances in satoshis and
// returns the equivalent balance(s) in the currency specified with
// two digits of precision.
func (w Watcher) ConvertBalance(currency string, balancesSat ...int) (bs []string, err error) {
	price, err := w.BTCAPI.Price()
	if err != nil {
		return nil, fmt.Errorf("error calling btcapi: %v", err)
	}

	for _, b := range balancesSat {	
		balanceCurrency := 0.0
		bitcoinBalance := float64(b) / float64(SatsPerBitcoin)
		switch currency {
		case CurrencyUSD:
			balanceCurrency = price.USD * bitcoinBalance
		case CurrencyEUR:
			balanceCurrency = price.EUR * bitcoinBalance
		case CurrencyGBP:
			balanceCurrency = price.GBP * bitcoinBalance
		case CurrencyXAU:
			balanceCurrency = price.XAU * bitcoinBalance
		default:
			balanceCurrency = price.USD * bitcoinBalance
		}
		bs = append(bs, fmt.Sprint(math.Round(balanceCurrency*100)/100))
	}
	// Round with two digits of precision
	return bs, nil
}
