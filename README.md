# FireFly Performance CLI

FireFly Performance CLI is a HTTP load testing tool that leverages [Vegeta](https://github.com/tsenart/vegeta) to generate a constant request rate against a [FireFly](https://github.com/hyperledger/firefly) network and measure performance. This it to be confident [FireFly](https://github.com/hyperledger/firefly) can perform under normal conditions for an extended period of time.

## Items Subject to Testing

- [x] Broadcasts (`POST /messages/broadcasts`)
- [x] Private Messaging (`POST /messages/private`)
- [x] Mint Tokens (`POST /tokens/mint`)
- [x] Transfer Tokens (`POST /tokens/transfer`)
- [x] Burn Tokens (`POST /tokens/burn`)
- [ ] Fungible vs. Non-Fungible Token Toggle
- [ ] Mint/Transfer/Burn Token with message

## Run a test

`ff-perf -n http://localhost:5000`

## Options

```shell
Usage:
  ff-perf [flags]

Flags:
  -d, --duration duration   Duration of test (seconds) (default 1m0s)
  -f, --frequency int       Requests Per Second (RPS) frequency (default 50)
  -h, --help                help for ff-perf
  -j, --jobs int            Number of jobs to run (default 100)
  -n, --node string         FireFly node endpoint
  -r, --recipient string    Recipient for FF messages
  -w, --workers int         Number of workers at a time (default 1)
```

## Architecture
<img width="2277" alt="Screen Shot 2022-01-03 at 2 49 10 PM" src="https://user-images.githubusercontent.com/17093033/147973861-422d583f-3a55-437c-b08a-46d214727855.png">
