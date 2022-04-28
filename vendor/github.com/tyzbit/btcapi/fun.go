package btcapi

import (
	"encoding/json"
	"fmt"
)

type QuoteDetails struct {
	Text    string `json:"text"`
	Speaker string `json:"speaker"`
	URL     string `json:"url"`
	Date    string `json:"date"`
	Context string `json:"context"`
}

const QuotesRoute string = "/quotes"

// Quotes returns all quotes
func (c Config) Quotes() (quotes []QuoteDetails, err error) {
	body, err := getAPI(c.ExplorerURL + api + QuotesRoute + "/all")
	if err != nil {
		return quotes, err
	}

	err = json.Unmarshal(body, &quotes)
	if err != nil {
		return quotes, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return quotes, nil
}

// Quote takes an index and returns the corresponding quote
func (c Config) Quote(index int) (quote QuoteDetails, err error) {
	body, err := getAPI(c.ExplorerURL + api + QuotesRoute + "/" + fmt.Sprintf("%d", index))
	if err != nil {
		return quote, err
	}

	err = json.Unmarshal(body, &quote)
	if err != nil {
		return quote, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return quote, nil
}

// RandomQuote takes an index and returns the corresponding quote
func (c Config) RandomQuote() (quote QuoteDetails, err error) {
	body, err := getAPI(c.ExplorerURL + api + QuotesRoute + "/random")
	if err != nil {
		return quote, err
	}

	err = json.Unmarshal(body, &quote)
	if err != nil {
		return quote, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return quote, nil
}
