package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// IdentifierPOST is used to get balances
// and delete identifiers (addresses and pubkeys)
type IdentifierPOST struct {
	Identifier string `json:"identifier"`
}

// AddWatchPOST is used to begin watching
// an identifier (address or pubkey)
type AddWatchPOST struct {
	Identifier string `json:"identifier"`
	Nickname   string `json:"nickname"`
}

// AddWatchResponse is the response from an
// AddWatch request
type AddWatchResponse struct {
	Errors string `json:"errors"`
}

// BalanceResponse is the response from a
// GetBalance request
type BalanceResponse struct {
	Errors      string      `json:"errors,omitempty"`
	BalanceInfo interface{} `json:"reqInfo,omitempty"`
}

// BalancesResponse is the response from a
// GetBalances request
type BalancesResponse struct {
	Addresses []AddressInfo `json:"addresses,omitempty"`
	Pubkeys   []PubkeyInfo  `json:"pubkeys,omitempty"`
}

// GetWatchesResponse is the response from a
// GetWatches request
type GetWatchesResponse []Watches

// Watches is an object representing a single
// watched identifier (address or pubkey)
type Watches struct {
	Identifier string `json:"address"`
	Nickname   string `json:"nickname"`
}

// IsPubkey returns boolean if the string passed looks like a pubkey
func IsPubkey(i string) bool {
	return strings.HasPrefix(i, "xpub") ||
		strings.HasPrefix(i, "ypub") ||
		strings.HasPrefix(i, "zpub")
}

// AddWatch adds an identifier to be watched (address or pubkey)
func (w Watcher) AddWatch(c *gin.Context) {
	status := http.StatusCreated
	body, _ := ioutil.ReadAll(c.Request.Body)
	var req AddWatchPOST
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusInternalServerError, BalanceResponse{
			Errors: fmt.Sprint(err),
		})
		return
	}
	response := AddWatchResponse{}
	if IsPubkey(req.Identifier) {
		var oldPubkeyInfo PubkeyInfo
		w.DB.Model(&PubkeyInfo{}).
			Where(&PubkeyInfo{Pubkey: req.Identifier}).
			Scan(&oldPubkeyInfo)

		// Insert blank PubkeyInfo if none was found
		if (oldPubkeyInfo != PubkeyInfo{}) {
			status = http.StatusBadRequest
			response.Errors = "Pubkey is already being watched"
			c.JSON(status, response)
			return
		}
		if _, err := w.CreateNewPubkeyInfo(req.Identifier, req.Nickname); err != nil {
			status = http.StatusConflict
			response.Errors = fmt.Sprint(err)
			c.JSON(status, response)
			return
		}
		cancel := make(chan bool, 1)
		watcher.CancelWaitGroup.Add(1)
		w.AddCancelSignal(req.Identifier, cancel)
		go watcher.WatchPubkey(cancel, req.Identifier)
	} else {
		var oldAddressInfo AddressInfo
		w.DB.Model(&AddressInfo{}).
			Where(&AddressInfo{Address: req.Identifier}).
			Scan(&oldAddressInfo)

		// Insert blank AddressInfo if none was found
		if (oldAddressInfo != AddressInfo{}) {
			status = http.StatusConflict
			response.Errors = "Address is already being watched"
			c.JSON(status, response)
			return
		}
		if _, err := w.CreateNewAddressInfo(req.Identifier, req.Nickname); err != nil {
			status = http.StatusInternalServerError
			response.Errors = fmt.Sprint(err)
			c.JSON(status, response)
			return
		}
		cancel := make(chan bool, 1)
		watcher.CancelWaitGroup.Add(1)
		w.AddCancelSignal(req.Identifier, cancel)
		go watcher.WatchAddress(cancel, req.Identifier)
		c.JSON(status, response)
	}
}

// GetNickname gets the nickname of an identifier (address or pubkey)
func (w Watcher) GetNickname(id string) string {
	if IsPubkey(id) {
		pki := PubkeyInfo{}
		w.DB.Model(&PubkeyInfo{}).Where(&PubkeyInfo{Pubkey: id}).
			Scan(&pki)
		return pki.Nickname
	} else {
		addi := AddressInfo{}
		w.DB.Model(&AddressInfo{}).Where(&AddressInfo{Address: id}).
			Scan(&addi)
		return addi.Nickname
	}
}

// GetBalance gets the balance of an identifier (address or pubkey)
func (w Watcher) GetBalance(c *gin.Context) {
	body, _ := ioutil.ReadAll(c.Request.Body)
	var req IdentifierPOST
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusInternalServerError, BalanceResponse{
			Errors: fmt.Sprint(err),
		})
		return
	}

	status := http.StatusOK
	if IsPubkey(req.Identifier) {
		pubkeyInfo := w.GetPubkeyInfo(req.Identifier)
		if (pubkeyInfo == PubkeyInfo{}) {
			status = http.StatusNoContent
		}
		c.JSON(status, BalanceResponse{
			BalanceInfo: pubkeyInfo,
		})
	} else {
		addressInfo := w.GetAddressInfo(req.Identifier)
		if (addressInfo == AddressInfo{}) {
			status = http.StatusNoContent
		}
		c.JSON(status, BalanceResponse{
			BalanceInfo: addressInfo,
		})
	}
}

// GetBalances gets the balance of all known identifiers (addresses and pubkeys)
func (w Watcher) GetBalances(c *gin.Context) {
	status := http.StatusOK

	var a []AddressInfo
	var p []PubkeyInfo
	w.DB.Model(&AddressInfo{}).Scan(&a)
	w.DB.Model(&PubkeyInfo{}).Scan(&p)
	c.JSON(status, BalancesResponse{
		Addresses: a,
		Pubkeys:   p,
	})
}

// GetWatches returns all of the watched identifiers (addresses and pubkeys)
func (w Watcher) GetWatches(c *gin.Context) {
	status := http.StatusOK
	response := GetWatchesResponse{}
	for id := range w.CancelSignals {
		response = append(response, Watches{
			Identifier: id,
			Nickname:   w.GetNickname(id),
		})
	}
	if len(response) == 0 {
		status = http.StatusNotFound
	}
	c.JSON(status, response)
}

// DeleteIdentifier stops watching an identifier (address or pubkey)
// and removes it from the database
func (w Watcher) DeleteIdentifier(c *gin.Context) {
	body, _ := ioutil.ReadAll(c.Request.Body)
	var req IdentifierPOST
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusInternalServerError, BalanceResponse{
			Errors: fmt.Sprint(err),
		})
		return
	}

	status := http.StatusOK
	w.CancelWaitGroup.Add(1)
	w.DeleteCancelSignal(req.Identifier)
	if IsPubkey(req.Identifier) {
		c.JSON(status, w.DeletePubkeyInfo(req.Identifier))
	} else {
		c.JSON(status, w.DeleteAddressInfo(req.Identifier))
	}
}
