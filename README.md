# FireFly Performance CLI

FireFly Performance CLI is a HTTP load testing tool that generates a constant request rate against a [FireFly](https://github.com/hyperledger/firefly) network and measure performance. This it to be confident [FireFly](https://github.com/hyperledger/firefly) can perform under normal conditions for an extended period of time.

## Items Subject to Testing

- [x] Broadcasts (`POST /messages/broadcasts`)
- [x] Private Messaging (`POST /messages/private`)
- [x] Mint Tokens (`POST /tokens/mint`)
- [x] Fungible vs. Non-Fungible Token Toggle
- [ ] Blobs

## Run a test

`ffperf run-tests msg_broadcast -l 24h`

## Options

```shell
Usage:
  ffperf run-tests [flags]

Flags:
  -a, --address string            Address of custom contract
      --chaincode string          Chaincode name for custom contract
      --channel string            Fabric channel for custom contract
  -h, --help                      help for run-tests
  -l, --length duration           Length of entire performance test (default 1m0s)
      --longMessage               Include long string in message
  -r, --recipient string          Recipient for FireFly messages
  -x, --recipientAddress string   Recipient address for FireFly transfers
  -s, --stackJSON string          Path to stack.json file that describes the network to test
      --tokenType string          [fungible nonfungible] (default "fungible")
  -w, --workers int               Number of workers at a time (default 1)
)
```

## Examples

- 10 workers submit broadcast messages for 500 hours

  - `ffperf run-tests msg_broadcast -l 500h -w 10`

- 75 workers submit broadcast messages, private messages, and token mints for 10 hours. 25 workers per item
  - `ffperf run-tests msg_broadcast msg_private token_mint -l 10h -r "0x123" -w 75`
