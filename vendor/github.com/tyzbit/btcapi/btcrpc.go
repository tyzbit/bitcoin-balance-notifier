package btcapi

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type Config struct {
	ExplorerURL string
}

const api = "/api"

func getAPI(url string) ([]byte, error) {
	client := http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return []byte{}, fmt.Errorf("unable to call url %v, err: %w", url, err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("error reading body from api: %w", err)
	}
	return body, nil
}
