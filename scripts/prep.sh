#!/bin/bash
# Colors
GREEN='\033[1;32m'
PURPLE='\033[0;35m'
RED='\033[0;31m'
NC='\033[0m' # No Color

BASE_PATH=~/ffperf-testing

# Verify three arguments were given
if [ $# -ne 3 ]; then
    printf "${RED}Must provide exactly three arguments: \n1. Old stack name to remove \n2. New stack name to create \n3. Stack's blockchain type (ex. geth, besu, fabric, corda) \nex: ./prep.sh old_stack new_stack geth${NC}\n"
    exit 1
fi

OLD_STACK_NAME=$1
NEW_STACK_NAME=$2
BLOCKCHAIN_PROVIDER=$3

# Kill existing ffperf processes
printf "${PURPLE}Killing ffperf processes...\n${NC}"
pkill -f 'ffperf'
rm $BASE_PATH/ffperf.log

# Install local ffperf-cli
printf "${PURPLE}Installing local ffperf CLI...\n${NC}"
cd $BASE_PATH/firefly-perf-cli
make install

# Build firefly image
printf "${PURPLE}Building FireFly Image...\n${NC}"
cd $BASE_PATH/firefly
make docker

cd $BASE_PATH
# Remove old Firefly stack
printf "${PURPLE}Removing FireFly Stack: $OLD_STACK_NAME...\n${NC}"
ff remove -f $OLD_STACK_NAME

# Create new Firefly stack
printf "${PURPLE}Creating FireFly Stack: $NEW_STACK_NAME...\n${NC}"
ff init $NEW_STACK_NAME 2 --manifest $BASE_PATH/firefly/manifest.json -t erc1155 -d postgres -b $BLOCKCHAIN_PROVIDER --prometheus-enabled --block-period 1 --ethconnect-config ethconnect.yml --core-config core-config.yml

cat ~/.firefly/stacks/$NEW_STACK_NAME/init/docker-compose.yml | yq '
  .services.firefly_core_0.logging.options.max-file = "250" |
  .services.firefly_core_0.logging.options.max-size = "500m"
  ' > /tmp/docker-compose.yml && cp /tmp/docker-compose.yml ~/.firefly/stacks/$NEW_STACK_NAME/init/docker-compose.yml

cat ~/.firefly/stacks/$NEW_STACK_NAME/init/docker-compose.yml | yq '
  .services.firefly_core_1.logging.options.max-file = "250" |
  .services.firefly_core_1.logging.options.max-size = "500m"
  ' > /tmp/docker-compose.yml && cp /tmp/docker-compose.yml ~/.firefly/stacks/$NEW_STACK_NAME/init/docker-compose.yml

printf "${PURPLE}Starting FireFly Stack: $NEW_STACK_NAME...\n${NC}"
ff start $NEW_STACK_NAME --verbose --no-rollback

# Get org identity
FIRST_ORG_IDENTITY=$(curl http://localhost:5000/api/v1/network/organizations | jq -r '.[0].did')
FIRST_ORG_ADDRESS=$(cat ~/.firefly/stacks/$NEW_STACK_NAME/stack.json | jq -r '.members[0].account.address')
FIRST_NODE_IDENTITY=$(curl http://localhost:5000/api/v1/network/nodes | jq -r '.[0].did')

SECOND_ORG_IDENTITY=$(curl http://localhost:5000/api/v1/network/organizations | jq -r '.[1].did')
SECOND_ORG_ADDRESS=$(cat ~/.firefly/stacks/$NEW_STACK_NAME/stack.json | jq -r '.members[1].account.address')
SECOND_NODE_IDENTITY=$(curl http://localhost:5000/api/v1/network/nodes | jq -r '.[1].did')

cd $BASE_PATH

printf ${PURPLE}"Deploying custom test contract...\n${NC}"

TESTS='"msg_broadcast","msg_private","blob_broadcast","blob_private"'

if [ "$BLOCKCHAIN_PROVIDER" == "geth" ]; then
    output=$(ff deploy ethereum $NEW_STACK_NAME ./firefly/test/data/simplestorage/simple_storage.json | jq -r '.address')
    prefix='contract address: '
    CONTRACT_ADDRESS=${output#"$prefix"}
    FLAGS="$FLAGS -a $CONTRACT_ADDRESS"
    TESTS="${TESTS},\"custom_ethereum_contract\",\"token_mint\""
    CONTRACT_OPTIONS="{\"address\": \"${CONTRACT_ADDRESS}\"}"
elif [ "$BLOCKCHAIN_PROVIDER" == "fabric" ]; then
    docker run --rm -v $BASE_PATH/firefly/test/data/assetcreator:/chaincode-go hyperledger/fabric-tools:2.4 peer lifecycle chaincode package /chaincode-go/package.tar.gz --path /chaincode-go --lang golang --label assetcreator
    output=$(ff deploy $NEW_STACK_NAME ./firefly/test/data/assetcreator/package.tar.gz firefly assetcreator 1.0)
    CONTRACT_OPTIONS="{\"channel\": \"firefly\", \"chaincode\": \"assetcreator\"}"
    TESTS="${TESTS},\"custom_fabric_contract\""
fi

cat <<EOF > $BASE_PATH/instances.yml
stackJSONPath: ${HOME}/.firefly/stacks/$NEW_STACK_NAME/stack.json

wsConfig:
  wsPath: /ws
  readBufferSize: 16000
  writeBufferSize: 16000
  initialDelay: 250ms
  maximumDelay: 30s
  initialConnectAttempts: 5
  heartbeatInterval: 5s

instances:
  - name: long-run
    tests: [${TESTS}]
    length: 500h
    sender: ${FIRST_NODE_IDENTITY}
    workers: 200
    messageOptions:
      longMessage: false
    tokenOptions:
      tokenType: fungible
    contractOptions: ${CONTRACT_OPTIONS}
EOF

echo "FLAGS=$FLAGS"

printf "${PURPLE}Modify $BASE_PATH/instances.yml and the commnd below and run...\n${NC}"

echo '```'
printf "${GREEN}nohup ffperf run -c $BASE_PATH/instances.yml -n long-run &> ffperf.log &${NC}\n"
echo '```'

echo "core-config.yml"
echo '```'
cat core-config.yml
echo '```'

echo "ethconnect.yml"
echo '```'
cat ethconnect.yml
echo '```'

echo "instances.yml"
echo '```'
cat instances.yml
echo '```'

echo "FireFly git commit:"
echo '```'
sh -c 'cd firefly; git rev-parse HEAD; cd ..'
echo '```'

# Create markdown for Perf Test
#printf "\n${RED}*** Before Starting Test ***${NC}\n"
#printf "${PURPLE}*** Add the following entry to https://github.com/hyperledger/firefly/issues/519 ***${NC}\n"
#printf "\n| $(uuidgen) | $(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p) | *Add Snapshot After Test* | $(TZ=":US/Eastern" date +%m_%d_%Y_%I_%M_%p) | *Add After Test* | *Add After Test* | $BLOCKCHAIN_PROVIDER | *Add Num Broadcasts* | *Add Num Private* | *Add Num Minters* | *Add Num On-Chain* | *Add related Github Issue* | $(cd $BASE_PATH/firefly;git rev-parse --short HEAD) | *Add After Test* | $(echo $(jq -r 'to_entries[] | "\(.key):\(.value .sha)"' $BASE_PATH/firefly/manifest.json)// )|\n"

