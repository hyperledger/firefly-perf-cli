# FireFly Performance CLI

FireFly Performance CLI is a HTTP load testing tool that generates a constant request rate against a [FireFly](https://github.com/hyperledger/firefly)
network and measure performance. This used to confirm confidence that [FireFly](https://github.com/hyperledger/firefly)
can perform under normal conditions for an extended period of time.

## Items Subject to Testing

- Broadcasts (`POST /messages/broadcasts`)
- Private Messaging (`POST /messages/private`)
- Mint Tokens (`POST /tokens/mint`)
  - Fungible vs. Non-Fungible Token Toggle
- Blobs
- Contract Invocation (`POST /contracts/invoke`)
  - Ethereum vs. Fabric

## Run

The test configuration is structured around running `ffperf` as either a single process or in a distributed fashion as
multiple processes.

In the test configuration you define one or more test _instances_ for a single `ffperf` process to run. An instance then
describes running one or more test _cases_ with a dedicated number of goroutine _workers_ against a _sender_ org and
a _recipient_ org. The test configuration consumes a reference the stack JSON configuration produced by the
[`ff` CLI](https://github.com/firefly-cli) (or can be defined manually) to understand the network topology, so that
sender's and recipient's just refer to indices within the stack.

As a result, running the CLI consists of providing an `instances.yaml` file describe the test configuration
and an instance index or name indicating which instance the process should run:

```shell
ffperf run -c /path/to/instances.yaml -i 0
```

See [`example-instances.yaml`](config/example-instances.yaml) for examples of how to define multiple instances
and multiple test cases per instance with all the various options.

## Options

```
Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network

Usage:
  ffperf run [flags]

Flags:
  -c, --config string          Path to performance config that describes the network and test instances
  -d, --daemon                 Run in long-lived, daemon mode. Any provided test length is ignored.
      --delinquent string      Action to take when delinquent messages are detected. Valid options: [exit log] (default "exit")
  -h, --help                   help for run
  -i, --instance-idx int       Index of the instance within performance config to run against the network (default -1)
  -n, --instance-name string   Instance within performance config to run against the network
)
```

## Distributed Deployment

See the [`ffperf` Helm chart](charts/ffperf) for running multiple instances of `ffperf` using Kubernetes.
