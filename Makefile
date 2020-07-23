# Copyright 2020 The Kubernetes Authors.
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

WHAT ?= ./...

DOCKER_REPO ?= gcr.io/k8s-staging-k8s-gsm-tools
DOCKER_TAG ?= v$(shell date -u '+%Y%m%d')-$(shell git describe --tags --always --dirty)

GCP_PROJECT ?= k8s-jkns-gke-soak

CMDS = $(notdir $(shell find ./cmd/ -maxdepth 1 -type d | sort))

.PHONY: all
all: build

.PHONY: $(CMDS)
$(CMDS):
	go build ./cmd/$@

.PHONY: build
build:
	go build $(WHAT)

.PHONY: test
test:
	go test -v $(WHAT)

.PHONY: test-e2e
test-e2e:
	go test -v --e2e-client --gsm-project=$(GCP_PROJECT) $(WHAT)

.PHONY: images
images: $(patsubst %,%-image,$(CMDS))

.PHONY: %-image
%-image:
	DOCKER_REPO=$(DOCKER_REPO) DOCKER_TAG=$(DOCKER_TAG) ./images/build.sh $*
