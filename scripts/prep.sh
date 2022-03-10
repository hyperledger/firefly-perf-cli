#!/bin/bash
# Colors
GREEN='\033[1;32m'
PURPLE='\033[0;35m'
RED='\033[0;31m'
NC='\033[0m' # No Color

BASE_PATH=~/ff-perf-testing

# Verify three arguments were given
if [ $# -ne 3 ]; then
    printf "${RED}Must provide exactly three arguments: \n1. Old stack name to remove \n2. New stack name to create \n3. Stack's blockchain type (ex. geth, besu, fabric, corda) \nex: ./prep.sh old_stack new_stack geth${NC}\n"
    exit 1
fi

# Kill existing ff-perf processes
printf "${PURPLE}Killing ff-perf processes...\n${NC}"
pkill -f 'ff-perf'
rm $BASE_PATH/ff-perf.log

# Install local ff-perf-cli
printf "${PURPLE}Installing local ff-perf-cli...\n${NC}"
cd $BASE_PATH/firefly-perf-cli
go install ./ff-perf

# Build firefly image
printf "${PURPLE}Building FireFly Image...\n${NC}"
cd $BASE_PATH/firefly
make docker

cd $BASE_PATH
# Remove old Firefly stack
printf "${PURPLE}Removing FireFly Stack: $1...\n${NC}"
ff remove -f $1

# Create new Firefly stack
printf "${PURPLE}Creating FireFly Stack: $2...\n${NC}"
ff init $2 2 --manifest $BASE_PATH/firefly/manifest.json -t erc1155 -d postgres -b $3 --prometheus-enabled
cat ~/.firefly/stacks/$2/docker-compose.yml | yq '
  .services.firefly_core_0.logging.options.max-file = "250" |
  .services.firefly_core_0.logging.options.max-size = "500m"
  ' > /tmp/docker-compose.yml && cp /tmp/docker-compose.yml ~/.firefly/stacks/$2/docker-compose.yml

printf "${PURPLE}Starting FireFly Stack: $2...\n${NC}"
ff start $2

# Get org identity
ORG_IDENTITY=$(curl http://localhost:5000/api/v1/network/organizations | jq -r '.[0].did')
ORG_ADDRESS=$(cat ~/.firefly/stacks/$2/stack.json | jq -r '.members[0].address')
cd $BASE_PATH

printf ${PURPLE}"Deploying custom test contract...\n${NC}"

prefix='contract address: '
output=$(ff deploy $2 ./firefly/test/data/simplestorage/simple_storage.json | grep address)
CONTRACT_ADDRESS=${output#"$prefix"}

printf "${PURPLE}Modify the command below and run...\n${NC}"
printf "${GREEN}nohup ff-perf msg_broadcast msg_private token_mint custom_contract -l 500h -r \"$ORG_IDENTITY\" -x \"$ORG_ADDRESS\" -w 100 -a $CONTRACT_ADDRESS -s ~/.firefly/stacks/$2/stack.json &> ff-perf.log &${NC}\n"

# Create markdown for Perf Test
printf "\n${RED}*** Before Starting Test ***${NC}\n"
printf "${PURPLE}*** Add the following entry to https://github.com/hyperledger/firefly/issues/519 ***${NC}\n"
printf "\n| $(uuidgen) | $(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p) | *Add Snapshot After Test* | $(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p) | *Add After Test* | *Add After Test* | $3 | *Add Num Broadcasts* | *Add Num Private* | *Add Num Minters* | *Add Num On-Chain* | *Add related Github Issue* | $(cd $BASE_PATH/firefly;git rev-parse --short HEAD) | *Add After Test* | $(echo $(jq -r 'to_entries[] | "\(.key):\(.value .sha)"' $BASE_PATH/firefly/manifest.json)// )|\n"