# FireFly-Perf-CLI Quick Start

## File Structure Setup

- Create a folder structure as below:

  ```bash
  ~/ffperf-testing/
  ├
  ├── firefly
  │   ├── <Firefly Files>
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
  git clone git@github.com:hyperledger/firefly.git ~/ffperf-testing/firefly
  git clone git@github.com:hyperledger/firefly-perf-cli.git ~/ffperf-testing/firefly-perf-cli
  cp hack/getLogs.sh ~/ffperf-testing/getLogs.sh
  cp hack/prep.sh ~/ffperf-testing/prep.sh
  ```

## Preparing Environment

### Checkout firefly branch you want to test

```bash
cd ~/ffperf-testing/firefly
git checkout ...
```

### Checkout ff-perf-cli branch you want to use for test

```bash
cd ~/ffperf-testing/firefly-perf-cli
git checkout ...
```

## Preparing Test

### Run script to setup test

- `./prep.sh` is a script that does the following:

  1. Kills existing perf test
  2. Builds local FireFly image
  3. Installs local FireFly Perf CLI
  4. Removes old FireFly stack
  5. Creates new FireFly stack
  6. Allows you to edit `docker-compose.yml`
  7. Starts FireFly stack
  8. Outputs command to kick off test

- Run:
  ```bash
  cd ~/ffperf-testing
  ./prep.sh <old_stack_name> <new_stack_name> <blockchain_type>
  # ex: ./prep.sh oldStack newStack geth
  ```
- Once script is done, you'll be given a command to run the perf test. Paste that in to the terminal to start the test.
- \*\*Please save the markdown row that is printed. You will need to use this to create an issue in https://github.com/hyperledger/firefly/issues/519

## Getting Logs

- `./getLogs.sh` is a script that does the following:

  1. Finds location of firefly_core, ethconnect, and ffperf logs
  2. Stores the compressed logs in a timestamped directory in ~/ff-perf-testing
  3. ```bash
     ~/ffperf-testing/
     ├── /ff_logs_03_03_2022_01_18_PM/
     │   ├── ffperf.log
     │   ├── log_ethconnect_0.log.gz
     │   └── log_firefly_core_0.log.gz
     ```

- Run:

  ```bash
  cd ~/ffperf-testing
  ./getLogs.sh <stack_name>
  # ex: ./getLogs.sh test
  ```

## Viewing Grafana

- In Grafana, add a Prometheus Data Source that points to your firefly node's address on port 9090 (ex. localhost:9090)
- Import `grafanaDashboard.json` in Grafana
- If you don't have Grafana on your machine, you can use the docs found [here](https://grafana.com/docs/grafana/latest/installation/)
