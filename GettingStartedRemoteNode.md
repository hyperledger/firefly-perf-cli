# FireFly-Perf-CLI Quick Start

These instructions will help you get started with running performance tests against a FireFly node running on a remote system.

The tests available to run against a remote FireFly node are more limited because the test cannot setup all of the FireFly components required for more involved tests, such as messaging between multiple instances of FireFly in a consortium.

However, they can be used to submit token mint transactions to remote FireFly instance which can be very useful to performance test the overall throughput of the system. The example configuration files [example-remote-node-instances-nonfungible.yaml](config/example-remote-node-instances-nonfungible.yaml) and [example-remote-node-instances-fungible.yaml](config/example-remote-node-instances-fungible.yaml)) show how to configure the test client to submit NFT mint transactions or ERC20 mint transactions respectively. Follow the [manual run](#option-1:-manually-run-the-test) steps below to quickly run the performance client using an existing configuration file.

Note: you must create a FireFly token pool before running either of the remote examples. Modify the example and set the name of the FireFly token pool you want to test. You will also need to set the signing key to a key that FireFly has access to sign with.

## File Structure Setup

- Create a folder structure as below:

  ```bash
  ~/ffperf-testing/
  ├
  ├── firefly-perf-cli
  │   ├── <Firefly Perf CLI Files>
  ├
  ├── prep.sh
  ├── getLogs.sh
  ```

- Helper commands to make folder structure from above:
  ```bash
  # From the firefly-perf-cli folder
  mkdir ~/ffperf-testing
  git clone git@github.com:hyperledger/firefly-perf-cli.git ~/ffperf-testing/firefly-perf-cli
  cp scripts/getLogs.sh ~/ffperf-testing/getLogs.sh
  cp scripts/prepForRemote.sh ~/ffperf-testing/prepForRemote.sh
  ```

## Preparing Environment

### Checkout ff-perf-cli branch you want to use for test

```bash
cd ~/ffperf-testing/firefly-perf-cli
git checkout ...
```

## Preparing Test

### Option 1: Manually run the test

If you have a configuration file (such as the remote NFT test file [here](config/example-remote-node-instances-nonfungible.yaml) or remote ERC20 test file [here](config/example-remote-node-instances-fungible.yaml))
you can start and stop the test manually without running the prep script.

Simply run the `ffperf` command with the location of the configuration file and the name of the test to run from that file, for example:

```
ffperf run -c <location-of-instances.yml> -n test1
```

### Option 2: Run script to setup test

This option helps to run everything needed for the test. The prep script takes input parameters that it uses to generate an `instances.yml` configuration file. The script only supports a subset of the
most common test options. To use the advanced test options you will need to modify the `instances.yml` file after it has been generated.

- `./prepForRemote.sh` is a script that does the following:

  1. Kills existing perf test
  2. Installs local FireFly Perf CLI
  3. Starts a local prometheus container configured to consume metrics from ffperf
  4. Sets up a configuration file based on the provided arguments
  5. Outputs command to kick off test

- Run:
  ```bash
  cd ~/ffperf-testing
  ./prepForRemote.sh
  # ex: ./prepForRemote.sh https://my.firefly.node/ my/prefix myuser mypassw0rd mynamespace 0xe7ca06eabfb44ef4d8de55fe2b4e0e42661b7209 mytoken 45659 0xa049c3dc2a7015a6188b2dd061fd519136c725bc
  ```
- Once script is done, you'll be given a command to run the perf test. Paste that in to the terminal to start the test.

## Getting Logs

- `./getPerfClientLogs.sh` is a script that does the following:

  2. Stores the compressed ffperf logs in a timestamped directory in ~/ff-perf-testing
  3. ```bash
     ~/ffperf-testing/
     ├── /ff_logs_03_03_2022_01_18_PM/
     │   ├── ffperf.log
     ```

- Run:

  ```bash
  cd ~/ffperf-testing
  ./getPerfClientLogs.sh
  ```

## Viewing Grafana

- In Grafana, add a Prometheus Data Source that points to the prometheus container that was started by the test your firefly node's address on port 9090 (ex. localhost:9090)
- Import `grafanaDashboard.json` in Grafana
- If you don't have Grafana on your machine, you can use the docs found [here](https://grafana.com/docs/grafana/latest/installation/)
