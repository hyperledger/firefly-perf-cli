#!/bin/bash
BLUE='\033[1;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

if [ $# -ne 1 ]; then
    printf "${RED}Must provide stack name as argument${NC}\n"
    exit 1
fi
STACK_NAME=$1

cd ~/ffperf-testing

# Create folder with timestamp
DIRNAME="ff_logs_$(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p)"
mkdir ~/ffperf-testing/$DIRNAME

# Copy ffperf logs
tail -n 100000 ~/ffperf-testing/ffperf.log > ~/ffperf-testing/$DIRNAME/ffperf_l100k.txt

function getLogs() {
    local container_name = $1
    local log_path=$(docker inspect --format='{{.LogPath}}' "$STACK_NAME_$container_name")
    local log_file_base=$(basename $log_path)
    # Get the base log file
    sudo cp $log_path ./$DIRNAME/log_${container_name}.log
    sudo chmod 777 ./$DIRNAME/log_${container_name}.log
    gzip ./$DIRNAME/log_${container_name}.log
    # Get one historical log file too
    sudo cp $log_path.1 ./$DIRNAME/log_${container_name}.log.1
    sudo chmod 777 ./$DIRNAME/log_${container_name}.log.1
    gzip ./$DIRNAME/log_${container_name}.log.1
}


getLogs 'firefly_core_0'
getLogs 'firefly_core_1'
getLogs 'ethconnect_0'
getLogs 'ethconnect_1'
