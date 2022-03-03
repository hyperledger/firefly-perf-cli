#!/bin/bash
BLUE='\033[1;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

if [ $# -ne 1 ]; then
    printf "${RED}Must provide stack name as argument${NC}\n"
    exit 1
fi

cd ~/ff-perf-testing

# Create folder with timestamp
DIRNAME="ff_logs_$(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p)"
mkdir ~/ff-perf-testing/$DIRNAME

# Copy ff-perf logs
cp ~/ff-perf-testing/ff-perf.log ~/ff-perf-testing/$DIRNAME

# Fetch firefly_core_0 logs
LOG_PATH=$(docker inspect --format='{{.LogPath}}' "$1_firefly_core_0")
LOG_FILE=$(basename $LOG_PATH)
sudo cp $LOG_PATH ./$DIRNAME/log_firefly_core_0.log
sudo chmod 777 ./$DIRNAME/log_firefly_core_0.log
gzip ./$DIRNAME/log_firefly_core_0.log

# Fetch ethconnect_0 logs
LOG_PATH=$(docker inspect --format='{{.LogPath}}' "$1_ethconnect_0")
LOG_FILE=$(basename $LOG_PATH)
sudo cp $LOG_PATH ./$DIRNAME/log_ethconnect_0.log
sudo chmod 777 ./$DIRNAME/log_ethconnect_0.log
gzip ./$DIRNAME/log_ethconnect_0.log