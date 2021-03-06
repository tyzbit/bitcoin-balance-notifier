package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tyzbit/btcapi"
)

// PubkeyInfo represents information about a given pubkey.
// Pubkey implements the Info interface
type PubkeyInfo struct {
	Pubkey                  string `gorm:"primaryKey"`
	Nickname                string
	BalanceSat              int
	PreviousBalanceSat      int
	Currency                string
	BalanceCurrency         string
	PreviousBalanceCurrency string
	TXCount                 int
}

// GetIdentifier returns the pubkey of the PubkeyInfo variable
func (p PubkeyInfo) GetIdentifier() string {
	return p.Pubkey
}

// GetNickname returns the nickname of the PubkeyInfo variable
func (p PubkeyInfo) GetNickname() string {
	return p.Nickname
}

// Update updates the database with the PubkeyInfo variable
func (p PubkeyInfo) Update(w Watcher) error {
	tx := w.DB.Model(&PubkeyInfo{}).
		Where(&PubkeyInfo{Pubkey: p.Pubkey, Nickname: p.Nickname}).
		Updates(&p)
	if tx.RowsAffected != 1 {
		return fmt.Errorf("%d rows affected", tx.RowsAffected)
	}
	return nil
}

const (
	pubkeyMessageTemplate = `**Pubkey Balance Changed**
Nickname: {{ .Nickname }}
Address: {{ .Pubkey }}
Previous Balance (satoshis): {{ .PreviousBalanceSat }}
Previous Balance ({{ .Currency }}): {{ .PreviousBalanceCurrency }}
Transactions: {{ .TXCount }}
New Balance (satoshis): {{ .BalanceSat }}
New Balance ({{ .Currency }}): {{ .BalanceCurrency }}
`
)

// WatchPubkey takes a btcapi config and a nickname:pubkey string. It
// checks the database for a previous pubkey summary and compares the
// previous balance to the current balance. If they are different,
// it calls Watcher.SendNotification.
func (w Watcher) WatchPubkey(stop chan bool, pubkey string) {
main:
	for {
		select {
		case <-stop:
			break main
		default:
			nickname := w.GetNickname(pubkey)
			var pubKeys []string
			pubKeys = append(pubKeys, pubkey)
			oldPubkeyInfo := w.GetPubkeyInfo(pubKeys[0])
			// Insert a blank PubkeyInfo if none was found
			if (oldPubkeyInfo == PubkeyInfo{}) {
				var err error
				oldPubkeyInfo, err = w.CreateNewPubkeyInfo(pubKeys[0], nickname)
				if err != nil {
					log.Error(err)
					return
				}
			}

			pubkeySummary, err := w.BTCAPI.ExtendedPublicKeyDetails(pubKeys[0])
			if err != nil {
				log.Errorf("error calling btcapi: %v", err)
				continue
			}

			if w.CheckAllPubkeyTypes {
				for _, pubkeyType := range pubkeySummary.RelatedKeys {
					pubKeys = append(pubKeys, pubkeyType.Key)
				}
			}

			// totalBalance is the balance of all pubkeys, similar for totalTxCount.
			totalBalance, totalTxCount := 0, 0
			for _, pubkey := range pubKeys {
				// totalPubkeyBalance is the balance for this pubkey, similar
				// for totalPubkeyTxCount. NoTXCount is incremented when
				// a consecutive address check yields no new transactions.
				totalPubkeyBalance, totalPubkeyTxCount, NoTXCount := 0, 0, 0

			pubkey:
				for offset := 0; 0 == 0; offset = offset + w.PageSize {
					pubKeyPage, err := w.BTCAPI.ExtendedPublicKeyDetailsPage(pubkey, w.PageSize, offset)
					if err != nil {
						log.Errorf("error calling btcapi: %v", err)
					}

					// pubkeyTxCount is used to keep track of how many addresses
					// we find that don't have new transactions.
					pubkeyTxCount := 0
					addresses := []string{}

					// Zipper join addresses.
					// ChangeAddresses and ReceiveAddresses should be the same length.
					for i := 0; i < len(pubKeyPage.ChangeAddresses); i++ {
						addresses = append(addresses, pubKeyPage.ReceiveAddresses[i], pubKeyPage.ChangeAddresses[i])
					}
					for _, address := range addresses {
						// Check if we've received a stop message
						select {
						case <-stop:
							break main
						default:
							log.Debug("checking address: " + address)
							addressSummary, err := w.UpdatePubkeysTotal(address, &totalPubkeyBalance, &totalPubkeyTxCount)
							if err != nil {
								log.Errorf("error updating pubkey total: %v", err)
								continue
							}
							if pubkeyTxCount == addressSummary.TXHistory.TXCount {
								if NoTXCount > w.Lookahead*2 {
									// Stop paging, we haven't had an address with
									// transactions in w.Lookahead * 2 addresses.
									// (we multiply by 2 because we're checking
									// both receive and change addresses)
									break pubkey
								}
								NoTXCount++
							}
							// Set the pubkeyTxCount so we can compare it next run to
							// monitor if we're seeing activity on the addresses
							// we're scanning.
							pubkeyTxCount = addressSummary.TXHistory.TXCount
						}
					}
				}

				// We're done checking this pubkey, add the balance to
				// the totals. If w.CheckAllPubkeyTypes is on, we
				// might check other pubkeys after this.
				totalBalance = totalBalance + totalPubkeyBalance
				totalTxCount = totalTxCount + totalPubkeyTxCount
			}

			currencyBalance, err := w.ConvertBalance(oldPubkeyInfo.Currency, totalBalance)
			if err != nil || currencyBalance == nil {
				log.Errorf("unable to convert balance of %d to %s, err: %v", totalBalance, w.Currency, err)
				currencyBalance[0] = "0.00"
			}
			pubkeyInfo := PubkeyInfo{
				Pubkey:                  pubKeys[0],
				Nickname:                nickname,
				BalanceSat:              totalBalance,
				BalanceCurrency:         currencyBalance[0],
				Currency:                oldPubkeyInfo.Currency,
				PreviousBalanceSat:      oldPubkeyInfo.BalanceSat,
				PreviousBalanceCurrency: oldPubkeyInfo.BalanceCurrency,
				TXCount:                 totalTxCount,
			}
			if pubkeyInfo.BalanceSat != oldPubkeyInfo.BalanceSat {
				log.Debugf("\"%s\" (%s) balance updated from %d to %d sats", nickname, pubKeys[0], oldPubkeyInfo.BalanceSat, pubkeyInfo.BalanceSat)
				w.UpdateInfo(pubkeyInfo)
				w.SendNotification(pubkeyInfo, pubkeyMessageTemplate)
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

// CreateNewPubkeyInfo reates an PubkeyInfo database entry for a new
// pubkey & nickname combination.
func (w Watcher) CreateNewPubkeyInfo(pubkey string, nickname string) (PubkeyInfo, error) {
	log.Warnf("previous pubkey information for \"%s\" (%s) was not found, database will be updated", nickname, pubkey)
	pubkeyInfo := PubkeyInfo{
		Pubkey:                  pubkey,
		Nickname:                nickname,
		BalanceSat:              0,
		BalanceCurrency:         "0.00",
		Currency:                w.Currency,
		PreviousBalanceSat:      0,
		PreviousBalanceCurrency: "0.00",
		TXCount:                 0,
	}
	tx := w.DB.Model(&PubkeyInfo{}).Create(&pubkeyInfo)
	if tx.RowsAffected != 1 {
		return pubkeyInfo, fmt.Errorf("%d rows affected creating pubkey info for \"%s\" (%s), err: %w", tx.RowsAffected, nickname, pubkey, tx.Error)
	}
	return pubkeyInfo, nil
}

// UpdatePubkeysTotal takes an address and updates the totals of the pointers provided
// and returns the addressSummary.
func (w Watcher) UpdatePubkeysTotal(address string, totalPubkeyBalance *int, totalPubkeyTxCount *int) (btcapi.AddressSummary, error) {
	addressSummary, err := w.BTCAPI.AddressSummary(address)
	if err != nil {
		return addressSummary, err
	}
	*totalPubkeyBalance = *totalPubkeyBalance + addressSummary.TXHistory.BalanceSat
	*totalPubkeyTxCount = *totalPubkeyTxCount + addressSummary.TXHistory.TXCount
	return addressSummary, nil
}

// GetPubkeyInfo gets a PubkeyInfo object from the database identified by a pubkey
func (w Watcher) GetPubkeyInfo(pubkey string) (p PubkeyInfo) {
	w.DB.Model(&PubkeyInfo{}).
		Where(&PubkeyInfo{Pubkey: pubkey}).
		Scan(&p)
	// Update the object with current exchange rates
	if (p != PubkeyInfo{}) {
		bs, err := w.ConvertBalance(p.Currency, p.PreviousBalanceSat, p.BalanceSat)
		if err != nil || bs == nil {
			log.Errorf("error converting balance, err: %v", err)
			return p
		}
		p.PreviousBalanceCurrency = bs[0]
		p.BalanceCurrency = bs[1]
		// Update the currency data in the database
		w.UpdateInfo(p)
	}
	return p
}

// DeletePubkeyInfo deletes a PubkeyInfo object from the database with a pubkey
func (w Watcher) DeletePubkeyInfo(pubkey string) bool {
	tx := w.DB.Model(&PubkeyInfo{}).
		Where(&PubkeyInfo{Pubkey: pubkey}).
		Delete(&PubkeyInfo{Pubkey: pubkey})
	if tx.RowsAffected != 1 {
		return false
	}
	return true
}
