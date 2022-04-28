package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// AddressInfo represents information about a given address.
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

// GetIdentifier returns the address of the AddressInfo variable.
func (a AddressInfo) GetIdentifier() string {
	return a.Address
}

// GetNickname returns the nickname of the AddressInfo variable.
func (a AddressInfo) GetNickname() string {
	return a.Nickname
}

// Update updates the database with the AddressInfo variable.
func (a AddressInfo) Update(w Watcher) error {
	tx := w.DB.Model(&AddressInfo{}).
		Where(&AddressInfo{Address: a.Address, Nickname: a.Nickname}).
		Updates(&a)
	if tx.RowsAffected != 1 {
		return fmt.Errorf("%d rows affected", tx.RowsAffected)
	}
	return nil
}

const (
	addressMessageTemplate = `**Address Balance Changed**
Nickname: {{ .Nickname }}
Address: {{ .Address }}
Previous Balance (satoshis): {{ .PreviousBalanceSat }}
Previous Balance ({{ .Currency }}): {{ .PreviousBalanceFiat }}
Transactions: {{ .TXCount }}
New Balance (satoshis): {{ .BalanceSat }}
New Balance ({{ .Currency }}): {{ .BalanceFiat }}
`
)

// WatchAddress takes a btcapi config and a nickname:address string. It
// checks the database for a previous address summary and compares the
// previous balance to the current balance. If they are different,
// it calls Watcher.SendNotification.
func (w Watcher) WatchAddress(nickAddress string) {
	for {
		nickname := strings.Split(nickAddress, ":")[0]
		address := strings.Split(nickAddress, ":")[1]

		var oldAddressInfo AddressInfo
		w.DB.Model(&AddressInfo{}).
			Where(&AddressInfo{Address: address, Nickname: nickname}).
			Scan(&oldAddressInfo)

		// Insert blank AddressInfo if none was found
		if (oldAddressInfo == AddressInfo{}) {
			oldAddressInfo = w.CreateNewAddressInfo(address, nickname)
		}

		addressSummary, err := w.BTCAPI.AddressSummary(address)
		if err != nil {
			log.Errorf("error calling btcapi: %v", err)
		}

		balanceFiat, err := w.ConvertBalance(addressSummary.TXHistory.BalanceSat)
		if err != nil {
			log.Errorf("unable to convert balance of %d to %s, err: %v", addressSummary.TXHistory.BalanceSat, w.Currency, err)
		}
		addressInfo := AddressInfo{
			Address:             address,
			Nickname:            nickname,
			BalanceSat:          addressSummary.TXHistory.BalanceSat,
			BalanceFiat:         strconv.FormatFloat(balanceFiat, 'f', 2, 64),
			Currency:            strings.ToUpper(w.Currency),
			PreviousBalanceSat:  oldAddressInfo.BalanceSat,
			PreviousBalanceFiat: oldAddressInfo.BalanceFiat,
			TXCount:             addressSummary.TXHistory.TXCount,
		}

		if addressInfo.BalanceSat != oldAddressInfo.BalanceSat {
			log.Debugf("\"%s\" (%s) balance updated from %d to %d sats", nickname, address, oldAddressInfo.BalanceSat, addressInfo.BalanceSat)
			w.UpdateInfo(addressInfo)
			w.SendNotification(addressInfo, addressMessageTemplate)
		}
		time.Sleep(time.Duration(int(time.Second) * w.SleepInterval))
	}
}

// CreateNewAddressInfo creates an AddressInfo database entry for a new
// address & nickname combination.
func (w Watcher) CreateNewAddressInfo(address string, nickname string) AddressInfo {
	log.Warnf("previous address information for \"%s\" (%s) was not found, database will be updated", nickname, address)
	addressInfo := AddressInfo{
		Address:             address,
		Nickname:            nickname,
		BalanceSat:          0,
		BalanceFiat:         "0.0",
		Currency:            w.Currency,
		PreviousBalanceSat:  0,
		PreviousBalanceFiat: "0.0",
		TXCount:             0,
	}
	tx := w.DB.Model(&AddressInfo{}).Create(&addressInfo)
	if tx.RowsAffected != 1 {
		log.Errorf("%d rows affected creating address info for \"%s\" (%s)", tx.RowsAffected, nickname, address)
	}
	return addressInfo
}
