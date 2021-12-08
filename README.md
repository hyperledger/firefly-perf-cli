# FireFly Performance CLI

Powered by [Vegeta](https://github.com/tsenart/vegeta), FireFly Performance CLI is a HTTP load testing tool used to generate a constant request rate against a [FireFly](https://github.com/hyperledger/firefly) network and measure performance.

## FireFly Performance Tests Included
- [x] Broadcasts
- [x] Private Messaging
- [x] Minting Tokens
- [ ] Transferring Tokens
- [ ] Burning Tokens

## Results
Test results are a mix of Vegeta metrics and custom, FireFly domain specific, tests. For example, in broadcasts & private messages, we measure the duration to process transactions.

```
Requests      [total, rate, throughput]         3000, 50.02, 50.01
Duration      [total, attack, wait]             59.994s, 59.977s, 16.459ms
Latencies     [min, mean, 50, 90, 95, 99, max]  9.671ms, 32.417ms, 18.36ms, 33.254ms, 45.816ms, 597.197ms, 1.161s
Bytes In      [total, mean]                     1775683, 591.89
Bytes Out     [total, mean]                     372000, 124.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      202:3000
Error Set:
Elapsed time between last sent message and 0 pending transactions: 10.007021619s
```

## Run a test
`ff-perf -n http://localhost:500`

## Adjust frequency and duration
`ff-perf -n http://localhost:500 -f 70 -d 30s`