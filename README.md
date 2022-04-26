# bitcoin-balance-notifier
Notifies if the balance of a bitcoin address changes

## Configuration

Set some environment variables before launching, or add a `.env` file.

| Variable | Value(s) |
|:-|:-|
| ADDRESSES | A comma separated list of addresses to watch |
| BTC_RPC_API | (optional) The URL to an instance of BTC-RPC-Explorer. Default: `https://bitcoinexplorer.org` |
| DISCORD_WEBHOOK | The URL to a Discord Webhook to call when the balance changes |
| INTERVAL | (optional) The amount of time, in seconds, between checking the balance. Default: `300` (5 minutes) | 
| LOG_LEVEL | `trace`, `debug`, `info`, `warn`, `error` |

## Database

Data is stored in either `/db/addresses.sqlite` or `./addresses.sqlite` in the same directory as the executable.
If running in Docker or Kubernetes, set up a volume at `/db` to persist data.

## Development

Create a `.env` file with your configuration, at the bare minimum you need
An address to watch and a Discord webhook. Run it with `go run main.go`