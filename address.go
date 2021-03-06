package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// AddressInfo represents information about a given address.
// AddressInfo implements the Info interface
type AddressInfo struct {
	Address                 string `gorm:"primaryKey"`
	Nickname                string
	BalanceSat              int
	PreviousBalanceSat      int
	Currency                string
	BalanceCurrency         string
	PreviousBalanceCurrency string
	TXCount                 int
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
Previous Balance ({{ .Currency }}): {{ .PreviousBalanceCurrency }}
Transactions: {{ .TXCount }}
New Balance (satoshis): {{ .BalanceSat }}
New Balance ({{ .Currency }}): {{ .BalanceCurrency }}
`
)

// WatchAddress takes a btcapi config and a nickname:address string. It
// checks the database for a previous address summary and compares the
// previous balance to the current balance. If they are different,
// it calls Watcher.SendNotification.
func (w Watcher) WatchAddress(stop chan bool, address string) {
main:
	for {
		select {
		case <-stop:
			return
		default:
			nickname := w.GetNickname(address)
			oldAddressInfo := w.GetAddressInfo(address)
			// Insert blank AddressInfo if none was found
			if (oldAddressInfo == AddressInfo{}) {
				var err error
				oldAddressInfo, err = w.CreateNewAddressInfo(address, nickname)
				if err != nil {
					log.Error(err)
					return
				}
			}

			addressSummary, err := w.BTCAPI.AddressSummary(address)
			if err != nil {
				log.Errorf("error calling btcapi: %v", err)
			}

			currencyBalance, err := w.ConvertBalance(oldAddressInfo.Currency, addressSummary.TXHistory.BalanceSat)
			if err != nil || currencyBalance == nil {
				log.Errorf("unable to convert balance of %d to %s, err: %v", addressSummary.TXHistory.BalanceSat, w.Currency, err)
				currencyBalance[0] = "0.00"
			}
			addressInfo := AddressInfo{
				Address:                 address,
				Nickname:                nickname,
				BalanceSat:              addressSummary.TXHistory.BalanceSat,
				BalanceCurrency:         currencyBalance[0],
				Currency:                oldAddressInfo.Currency,
				PreviousBalanceSat:      oldAddressInfo.BalanceSat,
				PreviousBalanceCurrency: oldAddressInfo.BalanceCurrency,
				TXCount:                 addressSummary.TXHistory.TXCount,
			}

			if addressInfo.BalanceSat != oldAddressInfo.BalanceSat {
				log.Infof("\"%s\" (%s) balance updated from %d to %d sats", nickname, address, oldAddressInfo.BalanceSat, addressInfo.BalanceSat)
				w.UpdateInfo(addressInfo)
				w.SendNotification(addressInfo, addressMessageTemplate)
			}
			// Check every second for a stop signal
			for i := 0; i < w.SleepInterval; i++ {
				select {
				case <-stop:
					break main
				default:
					time.Sleep(time.Second)
				}
			}
		}
	}
}

// CreateNewAddressInfo creates an AddressInfo database entry for a new
// address & nickname combination.
func (w Watcher) CreateNewAddressInfo(address string, nickname string) (AddressInfo, error) {
	log.Warnf("previous address information for \"%s\" (%s) was not found, database will be updated", nickname, address)
	addressInfo := AddressInfo{
		Address:                 address,
		Nickname:                nickname,
		BalanceSat:              0,
		BalanceCurrency:         "0.00",
		Currency:                w.Currency,
		PreviousBalanceSat:      0,
		PreviousBalanceCurrency: "0.00",
		TXCount:                 0,
	}

	tx := w.DB.Model(&AddressInfo{}).Create(&addressInfo)
	if tx.RowsAffected != 1 {
		return addressInfo, fmt.Errorf("%d rows affected creating address info for \"%s\" (%s), err: %w", tx.RowsAffected, nickname, address, tx.Error)
	}
	return addressInfo, nil
}

// Gets an AddressInfo object from the database identified by an address
func (w Watcher) GetAddressInfo(address string) (a AddressInfo) {
	w.DB.Model(&AddressInfo{}).
		Where(&AddressInfo{Address: address}).
		Scan(&a)
	// Update the object with current exchange rates
	if (a != AddressInfo{}) {
		var err error
		bs, err := w.ConvertBalance(a.Currency, a.PreviousBalanceSat, a.BalanceSat)
		if err != nil || bs == nil {
			log.Errorf("error converting balance, err: %v", err)
			return a
		}
		a.PreviousBalanceCurrency = bs[0]
		a.BalanceCurrency = bs[1]
		// Update the currency data in the database
		w.UpdateInfo(a)
	}
	return a
}

// Deletes an AddressInfo object from the database with an address
func (w Watcher) DeleteAddressInfo(address string) bool {
	tx := w.DB.Model(&AddressInfo{}).
		Where(&AddressInfo{Address: address}).
		Delete(&AddressInfo{Address: address})
	if tx.RowsAffected != 1 {
		return false
	}
	return true
}
