#!/bin/bash
# Colors
GREEN='\033[1;32m'
PURPLE='\033[0;35m'
RED='\033[0;31m'
NC='\033[0m' # No Color

BASE_PATH=~/ffperf-testing

# Verify three arguments were given
if [ $# -ne 7 ]; then
    printf "${RED}Must provide exactly 6 arguments:\n"    
    printf "${RED}1. Remote endpoint API URL\n"   
    printf "${RED}2. Endpoint API prefix\n"
    printf "${RED}3. Endpoint basic auth user\n"
    printf "${RED}4. Endpoint basic auth password\n"
    printf "${RED}5. FireFly namespace\n"
    printf "${RED}6. Existing token pool name\n"
    printf "${RED}7. Transaction signing key\n"
    printf "${RED}ex: ./prepForRemote.sh https://my.firefly.node/ my/prefix myuser mypassw0rd mynamespace mytoken 0xa049c3dc2a7015a6188b2dd061fd519136c725bc${NC}\n"
    exit 1
fi

REMOTE_ENDPOINT=$1
REMOTE_ENDPOINT_API_PREFIX=$2
REMOTE_ENDPOINT_USER=$3
REMOTE_ENDPOINT_PASSWORD=$4
FIREFLY_NAMESPACE=$5
EXISTING_TOKEN_POOL_NAME=$6
SIGNING_KEY=$7

# Kill existing ffperf processes
printf "${PURPLE}Killing ffperf processes...\n${NC}"
pkill -f 'ffperf'
rm $BASE_PATH/ffperf.log

# Install local ffperf-cli
printf "${PURPLE}Installing local ffperf CLI...\n${NC}"
cd $BASE_PATH/firefly-perf-cli
make install

cd $BASE_PATH

TESTS='{"name": "token_mint", "workers":50}'

cat <<EOF > $BASE_PATH/instances.yml

wsConfig:
  wsPath: /ws
  readBufferSize: 16000
  writeBufferSize: 16000
  initialDelay: 250ms
  maximumDelay: 30s
  initialConnectAttempts: 5
  heartbeatInterval: 60s
  disableTLSVerification: true

nodes:
  - name: long-run-node
    apiEndpoint: ${REMOTE_ENDPOINT}
    authUsername: ${REMOTE_ENDPOINT_USER}
    authPassword: ${REMOTE_ENDPOINT_PASSWORD}

instances:
  - name: long-run
    manualNodeIndex: 0
    tests: [${TESTS}]
    fireflyNamespace: ${FIREFLY_NAMESPACE}
    apiPrefix: ${REMOTE_ENDPOINT_API_PREFIX}
    signingAddress: ${SIGNING_KEY}
    maxTimePerAction: 60s
    skipMintConfirmations: true
    delinquentAction: log
    length: 500h
    tokenOptions:
      tokenType: nonfungible
      mintRecipient: ${SIGNING_KEY}
      supportsData: false
      supportsURI: true
      existingPoolName: ${EXISTING_TOKEN_POOL_NAME}
EOF

cat <<EOF > start.sh
ffperf run -c $BASE_PATH/instances.yml -n long-run
EOF

echo "FLAGS=$FLAGS"

printf "${PURPLE}Modify $BASE_PATH/instances.yml and the commnd below and run...\n${NC}"

echo '```'
printf "${GREEN}nohup ./start.sh &> ffperf.log &${NC}\n"
echo '```'

echo "instances.yml"
echo '```'
cat $BASE_PATH/instances.yml
echo '```'

