#!/bin/bash
BLUE='\033[1;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color


cd ~/ffperf-testing

# Create folder with timestamp
DIRNAME="ff_logs_$(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p)"
mkdir ~/ffperf-testing/$DIRNAME

# Copy ffperf logs
tail -n 100000 ~/ffperf-testing/ffperf.log > ~/ffperf-testing/$DIRNAME/ffperf_l100k.txt
