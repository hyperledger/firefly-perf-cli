# FireFly Performance CLI

FireFly Performance CLI is a HTTP load testing tool that generates a constant request rate against a [FireFly](https://github.com/hyperledger/firefly) network and measure performance. This it to be confident [FireFly](https://github.com/hyperledger/firefly) can perform under normal conditions for an extended period of time.

## Items Subject to Testing

- [x] Broadcasts (`POST /messages/broadcasts`)
- [x] Private Messaging (`POST /messages/private`)
- [x] Mint Tokens (`POST /tokens/mint`)
- [x] Fungible vs. Non-Fungible Token Toggle

## Run a test

`ff-perf msg_broadcast -l 24h -n http://localhost:5000`

## Options

```shell
Usage:
  ff-perf [flags]

Commands:
  msg_broadcast, msg_private, token_mint

Flags:
  -h, --help               help for ff-perf
  -l, --length duration    Length of entire performance test (default 1m0s)
      --longMessage        Include long string in message
  -n, --node string        FireFly node endpoint to test (default "http://localhost:5000")
  -r, --recipient string   Recipient for FireFly messages
      --tokenType string   [fungible nonfungible] (default "fungible")
  -w, --workers int        Number of workers at a time (default 1)
```

## Examples

- 10 workers submit broadcast messages for 500 hours

  - `ff-perf msg_broadcast -l 500h -w 10`

- 75 workers submit broadcast messages, private messages, and token mints for 10 hours. 25 workers per item
  - `ff-perf msg_broadcast msg_private token_mint -l 10h -r "0x123" -w 75`
