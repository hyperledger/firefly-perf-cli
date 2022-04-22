# Copyright © 2022 Kaleido, Inc.
#
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VGO=go
GOBIN := $(shell $(VGO) env GOPATH)/bin
GITREF := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION := canary
LINT := $(GOBIN)/golangci-lint

all: build
build: ## Builds all go code
		cd ffperf && go build -ldflags="-X 'github.com/hyperledger/firefly-perf-cli/internal/version.Version=$(VERSION)' -X 'github.com/hyperledger/firefly-perf-cli/internal/version.Date=$(DATE)' -X 'github.com/hyperledger/firefly-perf-cli/internal/version.Commit=$(GITREF)'"
install: ## Installs the package
		cd ffperf && go install -ldflags="-X 'github.com/hyperledger/firefly-perf-cli/internal/version.Version=$(VERSION)' -X 'github.com/hyperledger/firefly-perf-cli/internal/version.Date=$(DATE)' -X 'github.com/hyperledger/firefly-perf-cli/internal/version.Commit=$(GITREF)'"

docker:
	docker build --platform linux/amd64 --build-arg BUILD_VERSION=$(VERSION) . -t ghcr.io/hyperledger/firefly-perf-cli

lint: ${LINT} ## Checks and reports lint errors
		GOGC=20 $(LINT) run -v --timeout 5m

${LINT}:
		$(VGO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest


help:   ## Show this help
	@echo 'usage: make [target] ...'
	@echo ''
	@echo 'targets:'
	@egrep '^(.+)\:\ .*##\ (.+)' ${MAKEFILE_LIST} | sed 's/:.*##/#/' | column -t -c 2 -s '#'