# FireFly-Perf-CLI Quick Start

These instructions will help you get started with running performance tests against a FireFly node running on a remote system.

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

### Run script to setup test

- `./prepForRemote.sh` is a script that does the following:

  1. Kills existing perf test
  2. Installs local FireFly Perf CLI
  3. Starts a local prometheus container configured to consume metrics from ffperf
  4. Outputs command to kick off test

- Run:
  ```bash
  cd ~/ffperf-testing
  ./prepForRemote.sh
  # ex: ./prepForRemote.sh https://my.firefly.node/ my/prefix myuser mypassw0rd mynamespace 0xe7ca06eabfb44ef4d8de55fe2b4e0e42661b7209 mytoken 45659 0xa049c3dc2a7015a6188b2dd061fd519136c725bc
  ```
- Once script is done, you'll be given a command to run the perf test. Paste that in to the terminal to start the test.

## Getting Logs

- `./getPerfCliLogs.sh` is a script that does the following:

  2. Stores the compressed ffperf logs in a timestamped directory in ~/ff-perf-testing
  3. ```bash
     ~/ffperf-testing/
     ├── /ff_logs_03_03_2022_01_18_PM/
     │   ├── ffperf.log
     ```

- Run:

  ```bash
  cd ~/ffperf-testing
  ./getPerfCliLogsLogs.sh
  ```

## Viewing Grafana

- In Grafana, add a Prometheus Data Source that points to the prometheus container that was started by the test your firefly node's address on port 9090 (ex. localhost:9090)
- Import `grafanaDashboard.json` in Grafana
- If you don't have Grafana on your machine, you can use the docs found [here](https://grafana.com/docs/grafana/latest/installation/)
