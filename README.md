# bitcoin-balance-notifier
Notifies if the balance of a bitcoin address changes. 
Supports addresses and Extended Pubkeys.

## Configuration

Set some environment variables before launching, or add a `.env` file.

| Variable | Value(s) |
|:-|:-|
| ADDRESSES | A comma separated list of `nickname:address` to watch (example: `Test Address:34rng4QwB5pHUbGDJw1JxjLwgEU8TQuEqv`) |
| BTC_RPC_API | (optional) The URL to an instance of BTC-RPC-Explorer. Default: `https://bitcoinexplorer.org` |
| CHECK_ALL_PUBKEY_TYPES | Whether or not to check the other types of a given pubkey (xpub, ypub, zpub) |
| CURRENCY | Fiat currency to display balance in |
| DISCORD_WEBHOOK | The URL to a Discord Webhook to call when the balance changes |
| LOG_LEVEL | `trace`, `debug`, `info`, `warn`, `error` |
| LOOKAHEAD | How many addresses with no activity before we consider a pubkey to be completely scanned. Default: `20` |
| PAGE_SIZE | How many addresses to request at once for PubKey-type addresses Default: `100` |
| PUBKEYS | A comma separated list of `nickname:pubkey` to watch (example: `Test Pubkey:xpub6EuV33a2DXxAhoJTRTnr8qnysu81AA4YHpLY6o8NiGkEJ8KADJ35T64eJsStWsmRf1xXkEANVjXFXnaUKbRtFwuSPCLfDdZwYNZToh4LBCd`) |
| SLEEP_INTERVAL | (optional) The amount of time, in seconds, between checking the balance. Default: `300` (5 minutes) | 

## Database

Data is stored in either `/db/addresses.sqlite` or `./addresses.sqlite` in the same directory as the executable.
If running in Docker or Kubernetes, set up a volume at `/db` to persist data.

## Development

Create a `.env` file with your configuration, at the bare minimum you need
an address to watch and a Discord webhook. Run it with `go run main.go`