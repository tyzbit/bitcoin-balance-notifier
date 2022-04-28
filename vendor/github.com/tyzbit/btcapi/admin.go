package btcapi

import (
	"fmt"
)

const VersionRoute string = "/version"

// ExtendedPublicKeyDetails returns details for an extended public key.
func (c Config) Version() (version string, err error) {
	url := c.ExplorerURL + api + VersionRoute
	body, err := getAPI(url)
	if err != nil {
		return version, err
	}

	version = string(body)
	if err != nil {
		return version, fmt.Errorf("unable to parse returned body: %w", err)
	}
	return version, nil
}
