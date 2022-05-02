# bitcoin-balance-notifier

Notifies if the balance of a bitcoin address changes.
Supports addresses and Extended Pubkeys.

# Usage

```bash
docker compose up --build
```

Then navigate to http://127.0.0.1:8000. If you run the image by itself, it listens on `80` by default.

Prebuilt images are also available: `docker.io/tyzbit/bitcoin-balance-notifier:latest`. You can also replace `latest` with a release version.

## Configuration

Set some environment variables before launching, or add a `.env` file.

| Variable               | Value(s)                                                                                                | Required           |
| :--------------------- | ------------------------------------------------------------------------------------------------------- | ------------------ |
| BTC_RPC_API            | (optional) The URL to an instance of BTC-RPC-Explorer. Default: `https://bitcoinexplorer.org`           | No, but encouraged |
| CHECK_ALL_PUBKEY_TYPES | Whether or not to check the other types of a given pubkey (xpub, ypub, zpub). Defaults to `false`       | No                 |
| CURRENCY               | Currency to display balance in (`USD`,`GBP`,`EUR`,`XAU`). Defaults to `USD`                             | No                 |
| DISCORD_WEBHOOK        | The URL to a Discord Webhook to call when the balance changes                                           | Yes                |
| LOG_LEVEL              | `trace`, `debug`, `info`, `warn`, `error`                                                               | No                 |
| LOOKAHEAD              | How many addresses with no activity before we consider a pubkey to be completely scanned. Default: `20` | No                 |
| PAGE_SIZE              | How many addresses to request at once for PubKey-type addresses. Default: `100`                         | No                 |
| PORT                   | What port to listen on. Default: `80`                                                                   | No                 |
| SLEEP_INTERVAL         | (optional) The amount of time, in seconds, between checking the balance. Default: `300` (5 minutes)     | No                 |

## Database

Data is stored in either `/db/addresses.sqlite` or `./addresses.sqlite` in the same directory as the executable.
If running in Docker or Kubernetes, set up a volume at `/db` to persist data.
