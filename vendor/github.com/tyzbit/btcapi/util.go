package btcapi

import (
	"encoding/json"
	"fmt"
)

type ExtendedPublicKeyDetails struct {
	KeyType          string       `json:"keyType"`
	OutputType       string       `json:"outputType"`
	OutputTypeDesc   string       `json:"outputTypeDesc"`
	BIP32Path        string       `json:"bip32Path"`
	RelatedKeys      []RelatedKey `json:"relatedKeys"`
	ReceiveAddresses []string     `json:"receiveAddresses"`
	ChangeAddresses  []string     `json:"changeAddresses"`
}

type RelatedKey struct {
	KeyType      string `json:"keyType"`
	Key          string `json:"key"`
	OutputType   string `json:"outputType"`
	FirstAddress string `json:"firstAddress"`
}

const UtilRoute string = "/util"

// ExtendedPublicKeyDetails returns details for an extended public key.
func (c Config) ExtendedPublicKeyDetails(pubkey string) (details ExtendedPublicKeyDetails, err error) {
	return c.ExtendedPublicKeyDetailsPage(pubkey, 20, 0)
}

// ExtendedPublicKeyDetailsPage returns details for an extended public key.
// Page through the results by specifying a page
func (c Config) ExtendedPublicKeyDetailsPage(pubkey string, limit int, offset int) (details ExtendedPublicKeyDetails, err error) {
	options := ""
	if offset != 0 || limit != 0 {
		options = fmt.Sprintf("?limit=%d&offset=%d", limit, offset)
	}
	url := c.ExplorerURL + api + UtilRoute + "/xyzpub/" + pubkey + options
	body, err := getAPI(url)
	if err != nil {
		return details, err
	}

	err = json.Unmarshal(body, &details)
	if err != nil {
		return details, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return details, nil
}
