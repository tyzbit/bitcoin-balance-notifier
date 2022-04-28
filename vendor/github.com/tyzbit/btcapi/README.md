# btcapi
Go wrapper around the btc-rpc-explorer API

## Compatible API Versions
This is compatible with API version `v1.1.0`. It may be backwards compatible but
if you have issues, consider upgrading your BTC-RPC-Explorer API (or find a newer
version to use as a backend)

## Usage Example

```golang
package main

import (
    "fmt"
    "os"

    "github.com/tyzbit/btcapi"
)

func main() {
    address := "34rng4QwB5pHUbGDJw1JxjLwgEU8TQuEqv"
    btc := btcapi.Config{
        ExplorerURL: "https://btc-rpc-explorer.example.com",
    }
    summary, err := btc.AddressSummary(address)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    fmt.Println("the balance of ", address, " is ", summary.TXHistory.BalanceSat, " satoshis")
}
```