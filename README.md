# FireFly Performance CLI

FireFly Performance CLI is a HTTP load testing tool that generates a constant request rate against a [FireFly](https://github.com/hyperledger/firefly)
network and measure performance. This is used to confirm confidence that [FireFly](https://github.com/hyperledger/firefly)
can perform under normal conditions for an extended period of time.

## Items Subject to Testing

- Broadcasts (`POST /messages/broadcasts`)
- Private Messaging (`POST /messages/private`)
- Mint Tokens (`POST /tokens/mint`)
  - Fungible vs. Non-Fungible Token Toggle
- Blobs
- Contract Invocation (`POST /contracts/invoke`)
  - Ethereum vs. Fabric

## Build

The `ffperf` CLI needs building before you can use it.

Run `make install` in the root directory to build and install the `ffperf` command.

## Run

The test configuration is structured around running `ffperf` as either a single process or in a distributed fashion as
multiple processes.

The tool has 2 basic modes of operation:

1. Run against a local FireFly stack
   - In this mode the `ffperf` tool loads information about the FireFly endpoint(s) to test by reading from a FireFly `stack.json` file on the local system. The location of the `stack.json` file is configured in the `instances.yaml` file by setting the `stackJSONPath` option.
2. Run against a remote Firefly node
   - In this mode the `ffperf` tool connects to a FireFly instance running on a different system. Since there won't be a FireFly `stack.json` on the system where `ffperf` is running the nodes to test must be configured in the `instances.yaml` file by settings the `Nodes` option.

### Local FireFly stack

See the [`Getting Started`](GettingStarted.md) guide for help running tests against a local stack.

In the test configuration you define one or more test _instances_ for a single `ffperf` process to run. An instance then
describes running one or more test _cases_ with a dedicated number of goroutine _workers_ against a _sender_ org and
a _recipient_ org. The test configuration consumes a file reference to the stack JSON configuration produced by the
[`ff` CLI](https://github.com/hyperledger/firefly-cli) (or can be defined manually) to understand the network topology, so that
sender's and recipient's just refer to indices within the stack.

As a result, running the CLI consists of providing an `instances.yaml` file describe the test configuration
and an instance index or name indicating which instance the process should run:

```shell
ffperf run -c /path/to/instances.yaml -i 0
```

See [`example-instances.yaml`](config/example-instances.yaml) for examples of how to define multiple instances
and multiple test cases per instance with all the various options.

### Remote FireFly node

See the [`Getting Started with Remote Nodes`](GettingStartedRemoteNode.md) guide for help running tests against a remote FireFly node.

In the test configuration you define one or more test _instances_ for a single `ffperf` process to run. An instance then
describes running one or more test _cases_ with a dedicated number of goroutine _workers_. Instead of setting a _sender_ org and
_recipient_ org (because there is no local FireFly `stack.json` to read) the instance must be configured to use a `Node` that has
been defined in `instances.yaml`.

Currently the types of test that can be run against a remote node are limited to those that only invoke a single endpoint. This makes
it most suitable for test types `token_mint`, `custom_ethereum_contract` and `custom_fabric_contract` since these don't need
responses to be received from other members of the FireFly network.

To provide authentication when authenticating against a node endpoint, you can provide either of the following credentials in the `instances.yaml` under each `node` entry:

- bearer token - set the access token as the `authToken` value
- basic auth - set the username and password as the `authUsername` and `authPassword` values

> `authToken` takes precedence over `authUsername` and `authPassword` values

As a result, running the CLI consists of providing an `instances.yaml` file describe the test configuration
and an instance index or name indicating which instance the process should run:

```shell
ffperf run -c /path/to/instances.yaml -i 0
```

See [`example-remote-node-instances-fungible.yaml`](config/example-remote-node-instances-fungible.yaml) and [`example-remote-node-instances-nonfungible.yaml`](config/example-remote-node-instances-nonfungible.yaml) for examples of how to define nodes manually
and configure test instances to use them.

## Command line options

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

## Metrics

The `ffperf` tool registers the following metrics for prometheus to consume:

- ffperf_runner_received_events_total
- ffperf_runner_incomplete_events_total
- ffperf_runner_sent_mints_total
- ffperf_runner_sent_mint_errors_total
- ffperf_running_mint_token_balance (gauge)
- ffperf_runner_deliquent_msgs_total
- ffperf_runner_perf_test_duration_seconds

## Useful features

The `ffperf` tool is designed to let you run various styles of test. The default behaviour for a local stack will exercise a local FireFly stack for 500 hours or until an error occurs. The `prep.sh` script will help you create and run this comprehensive test to validate a local installation of FireFly.

There are various options for creating your own customized tests. A full list of configuration options can be seen at [`conf.go`](internal/conf/conf.go) but some useful options are outlined below:

- Setting a maximum number of test actions
  - See `maxActions` attribute (defaults to `0` i.e. unlimited).
  - Once `maxActions` test actions (e.g. token mints) have taken place the test will shut down.
- Ending the test when an error occurs
  - See `delinquentAction` attribute (defaults to `exit`).
  - A value of `exit` causes the test to end if an error occurs. Set to `log` to simply log the error and continue the test.
- Set the maximum duration of the test
  - See `length` attribute.
  - Setting a test instance's `length` attribute to a time duration (e.g. `3h`) will cause the test to run for that long or until an error occurs (see `delinquentAction`).
  - Note this setting is ignored if the test is run in daemon mode (running the `ffperf` command with `-d` or `--daemon`, or setting the global `daemon` value to `true` in the `instances.yaml` file). In daemon mode the test will run until `maxActions` has been reached or an error has occurred and `delinquentActions` is set to true.
- Ramping up the rate of test actions (e.g. token mints)
  - See the `startRate`, `endRate` and `rateRampUpTime` attribute of a test instance.
  - All values default to `0` which has the effect of not limiting the rate of the test.
  - The test will allow at most `startRate` actions to happen per second. Over the period of `rateRampUpTime` seconds the allowed rate will increase linearly until `endRate` actions per seconds are reached. At this point the test will continue at `endRate` actions per second until the test finishes.
  - If `startRate` is the only value that is set, the test will run at that rate for the entire test.
- Waiting for events to be confirmed before doing the next submission
  - See `noWaitSubmission` (defaults to `false`).
  - When set to `true` each worker routine will perform its action (e.g. minting a token) and wait for confirmation of that event before doing its next action.
  - `maxSubmissionsPerSecond` can be used to control the maximum number of submissions per second to avoid overloading the system under test.
- Setting the features of a token being tested
  - See `supportsData` and `supportsURI` attributes of a test instance.
  - `supportsData` defaults to `true` since the sample token contract used by FireFly supports minting tokens with data. When set to `true` the message included in the mint transaction will include the ID of the worker routine and used to correlate received confirmation events.
  - `supportsURI` defaults to `true` for nonfungible tokens. This attribute is ignored for fungible token tests. If set to `true` the ID of a worker routine will be set in the URI and used to correlate received confirmation events.
  - If neither attribute is set to true any received confirmation events cannot be correlated with mint transactions. In this case the test behaves as if `noWaitSubmission` is set to `true`.
- Enabling batching of events from the WebSocket connecting to FireFly. This will increase the amount of events per WebSocket message and increase the throughput of the tests
  - Under instances set 
   ```
   subscriptionOptions:
      batch: true <- Enables Batching
      readAhead: 50 <-- How many events to get in a batch. i.e 50 events in one websocket message
      batchTimeout: 250ms <-- Timeout to wait for FireFly to send the next batch if it's not filled 
    ```
- Waiting at the end of the test for the minted token balance of the `mintRecipient` address to equal the expected value. Since a test might be run several times with the same address the test gets the balance at the beginning of the test, and then again at the end. The difference is expected to equal the value of `maxActions`. To enable this check set the `maxTokenBalanceWait` token option the length of time to wait for the balance to be reached. If `maxTokenBalanceWait` is not set the test will not check balances.
- Having a worker loop submit more than 1 action per loop by setting `actionsPerLoop` for the test. This can be helpful when you want to scale the number of actions done in parallel without having to scale the number of workers. The default value is `1` for this attribute. If setting to a value > `1` it is recommended to have `noWaitSubmission` to set `false`.

## Distributed Deployment

See the [`ffperf` Helm chart](charts/ffperf) for running multiple instances of `ffperf` using Kubernetes.
