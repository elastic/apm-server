APMBENCH_PATH ?= ../../systemtest/cmd/apmbench
APMBENCH_GOOS ?= linux
APMBENCH_GOARCH ?= amd64

TFVARS_SOURCE ?= terraform.tfvars.example

BENCHMARK_WARMUP_TIME ?= 5m
BENCHMARK_AGENTS ?= 64
BENCHMARK_COUNT ?= 6
BENCHMARK_TIME ?= 2m
BENCHMARK_RUN ?= Benchmark
BENCHMARK_RESULT ?= benchmark-result.txt
BENCHMARK_DETAILED ?= true
BENCHMARK_EVENT_RATE ?= 0/s

GOBENCH_INDEX ?= apmbench-v2
GOBENCH_USERNAME ?= admin
GOBENCH_PASSWORD ?= changeme
GOBENCH_HOST ?= http://localhost:9200
GOBENCH_DEFAULT_TAGS = apm_server_version=$(APM_SERVER_VERSION)

SSH_USER ?= ec2-user
SSH_OPTS ?= -o LogLevel=ERROR -o StrictHostKeyChecking=no
SSH_KEY ?= ~/.ssh/id_rsa_terraform
WORKER_IP = $(shell terraform output -raw public_ip)

SHELL = /bin/bash
.SHELLFLAGS = -o pipefail -c

# This profile will also be used by the Terraform provider.
export AWS_PROFILE ?= default

.default: all

.PHONY: all
all: $(SSH_KEY) terraform.tfvars apmbench auth apply

MAKEFILE_PATH:=$(abspath $(lastword ${MAKEFILE_LIST}))
MAKEFILE_DIR:=$(dir ${MAKEFILE_PATH})

include ${MAKEFILE_DIR}/../../go.mk

.PHONY: auth
auth: init-aws-profile
	@okta-awscli --profile $(AWS_PROFILE)

.PHONY: init-aws-profile
init-aws-profile: ~/.aws/credentials
	@grep $(AWS_PROFILE) ~/.aws/credentials > /dev/null || \
		echo "[$(AWS_PROFILE)]\naws_access_key_id = x\naws_secret_access_key = x" >> ~/.aws/credentials

~/.aws/credentials:
	@mkdir -p ~/.aws
	@touch $@

terraform.tfvars:
	@sed "s/USER/$(USER)/" $(TFVARS_SOURCE) > terraform.tfvars

.PHONY: apmbench
apmbench:
	@echo "-> Building apmbench..."
	@cd $(APMBENCH_PATH) && CGO_ENABLED=0 GOOS=$(APMBENCH_GOOS) GOARCH=$(APMBENCH_GOARCH) $(GO) build .

.PHONY: init
init:
	@terraform init

.PHONY: apply
apply:
	@terraform apply -auto-approve

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve

.PHONY: log-benckmark-profile
log-benckmark-profile:
	@echo "Running benchmarks..."
	@echo "Benchmark warmup time: $(BENCHMARK_WARMUP_TIME)"
	@echo "Benchmark agents: $(BENCHMARK_AGENTS)"
	@echo "Benchmark event rate: $(BENCHMARK_EVENT_RATE)"
	@echo "Benchmark count: $(BENCHMARK_COUNT)"
	@echo "Benchmark duration: $(BENCHMARK_TIME)"
	@echo "Benchmark run expression : $(BENCHMARK_RUN)"

.PHONY: run-benchmark
run-benchmark: log-benckmark-profile
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(WORKER_IP) ". .envrc && bin/apmbench -run='$(BENCHMARK_RUN)' \
	-benchtime=$(BENCHMARK_TIME) -count=$(BENCHMARK_COUNT) -warmup-time=$(BENCHMARK_WARMUP_TIME) \
	-agents=$(BENCHMARK_AGENTS) -detailed=$(BENCHMARK_DETAILED) -event-rate=$(BENCHMARK_EVENT_RATE)" 2>&1 | tee $(BENCHMARK_RESULT)

.PHONY: run-benchmark-autotuned
run-benchmark-autotuned:
	$(eval APM_SERVER_SIZE:=$(shell echo var.apm_server_size | terraform console | tr -d '"'| tr -d 'g'))
	@ $(MAKE) run-benchmark BENCHMARK_AGENTS=$(shell echo $$(( $(BENCHMARK_AGENTS) * $(APM_SERVER_SIZE) )) )

.PHONY: index-benchmark-results
index-benchmark-results: $(GOBENCH) _default-gobench-vars
	@cat $(BENCHMARK_RESULT) | $(GOBENCH) -es $(GOBENCH_HOST) -es-username $(GOBENCH_USERNAME) -es-password $(GOBENCH_PASSWORD) \
	-index $(GOBENCH_INDEX) -tag "$(GOBENCH_DEFAULT_TAGS),$(GOBENCH_TAGS)"

.PHONY: _default-gobench-vars
_default-gobench-vars:
# TODO(marclop) Update code below to use a foor loop, rather than copying the lines.
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),apm_server_size=$(shell echo var.apm_server_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),elasticsearch_size=$(shell echo var.elasticsearch_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),stack_version=$(shell echo var.stack_version | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),apm_server_zone_count=$(shell echo var.apm_server_zone_count | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),elasticsearch_zone_count=$(shell echo var.elasticsearch_zone_count | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),build_sha=$(shell curl -sL -H "Authorization: Bearer $(shell terraform output -raw apm_secret_token )" $(shell terraform output -raw apm_server_url ) | jq -r '.build_sha'))

$(SSH_KEY):
	@ssh-keygen -t rsa -b 4096 -C "$(USER)@elastic.co" -N "" -f $(SSH_KEY)

.PHONY: ssh
ssh:
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(WORKER_IP)

.PHONY: docker-override-committed-version
docker-override-committed-version:
	$(eval TAG := $(shell grep docker.elastic.co/ ../../docker-compose.yml |grep SNAPSHOT| cut -d ':' -f3|uniq))
	@echo 'export TF_VAR_docker_image_tag_override={"elasticsearch":"$(TAG)","kibana":"$(TAG)","apm":"$(TAG)"}'

.PHONY: cleanup-elasticsearch
cleanup-elasticsearch:
	$(eval ELASTICSEARCH_URL = $(shell terraform output elasticsearch_url))
	$(eval ELASTICSEARCH_USER = $(shell terraform output -raw elasticsearch_username))
	$(eval ELASTICSEARCH_PASS = $(shell terraform output -raw elasticsearch_password))
	$(eval APM_DATA_STREAMS = traces-apm*,metrics-apm*,logs-apm*)
	@ echo "-> Deleting APM Server data streams..."
	@ curl -u $(ELASTICSEARCH_USER):$(ELASTICSEARCH_PASS) -XDELETE $(ELASTICSEARCH_URL)/_data_stream/$(APM_DATA_STREAMS)
