APMBENCH_PATH ?= ../../systemtest/cmd/apmbench
APMBENCH_GOOS ?= linux
APMBENCH_GOARCH ?= amd64

MOXY_GOOS ?= linux
MOXY_GOARCH ?= amd64

APM_SERVER_GOOS ?= linux
APM_SERVER_GOARCH ?= amd64

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
SSH_OPTS ?= -o LogLevel=ERROR -o StrictHostKeyChecking=no -o ServerAliveInterval=60 -o ServerAliveCountMax=10
SSH_KEY ?= ~/.ssh/id_rsa_terraform
WORKER_IP = $(shell terraform output -raw public_ip)
APM_SERVER_IP = $(shell terraform output -raw apm_server_ip)

SHELL = /bin/bash
.SHELLFLAGS = -o pipefail -c

# This profile will also be used by the Terraform provider.

# export AWS_PROFILE if CI is not defined
ifeq ($(CI),)
	export AWS_PROFILE ?= elastic-observability
endif

.default: all

.PHONY: all
all: $(SSH_KEY) terraform.tfvars apmbench auth apply

MAKEFILE_PATH:=$(abspath $(lastword ${MAKEFILE_LIST}))
MAKEFILE_DIR:=$(dir ${MAKEFILE_PATH})
REPO_ROOT:=$(abspath ${MAKEFILE_DIR}/../../)

include ${MAKEFILE_DIR}/../../go.mk

.PHONY: auth
auth:
	@okta-aws-cli --profile $(AWS_PROFILE) --write-aws-credentials --short-user-agent

terraform.tfvars:
	@sed "s/USER/$(USER)/" $(TFVARS_SOURCE) > terraform.tfvars

.PHONY: apmbench
apmbench:
	@echo "-> Building apmbench..."
	@cd $(APMBENCH_PATH) && CGO_ENABLED=0 GOOS=$(APMBENCH_GOOS) GOARCH=$(APMBENCH_GOARCH) go build .

.PHONY: moxy
moxy:
	@echo "-> Building moxy..."
	@cd ../../tools && CGO_ENABLED=0 GOOS=$(MOXY_GOOS) GOARCH=$(MOXY_GOARCH) go build -o "../build" github.com/elastic/apm-perf/cmd/moxy

.PHONY: apm-server
apm-server:
	@cd ../.. && make build/apm-server-$(APM_SERVER_GOOS)-$(APM_SERVER_GOARCH) && mv build/apm-server-$(APM_SERVER_GOOS)-$(APM_SERVER_GOARCH) build/apm-server

.PHONY: init
init:
	@terraform init

.PHONY: apply
apply:
	@terraform apply -auto-approve

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve

cp-cpuprof:
	@[ "${BENCHMARK_CPU_OUT}" ] && scp $(SSH_OPTS) -i $(SSH_KEY) "$(SSH_USER)@$(WORKER_IP):./$(BENCHMARK_CPU_OUT)" $(BENCHMARK_CPU_OUT) || echo "skipping cpu out copy"

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
	-rewrite-timestamps -rewrite-ids \
	-benchtime=$(BENCHMARK_TIME) -count=$(BENCHMARK_COUNT) -warmup-time=$(BENCHMARK_WARMUP_TIME) \
	-agents=$(BENCHMARK_AGENTS) -detailed=$(BENCHMARK_DETAILED) -event-rate=$(BENCHMARK_EVENT_RATE) -cpuprofile=$(BENCHMARK_CPU_OUT)" 2>&1 | tee $(BENCHMARK_RESULT)

.PHONY: run-benchmark-autotuned
run-benchmark-autotuned:
	$(eval APM_SERVER_SIZE:=$(shell echo var.apm_server_size | terraform console | tr -d '"'| tr -d 'g'))
	@ $(MAKE) run-benchmark BENCHMARK_AGENTS=$(shell echo $$(( $(BENCHMARK_AGENTS) * $(APM_SERVER_SIZE) )) )

.PHONY: index-benchmark-results
index-benchmark-results: _default-gobench-vars
	@cat $(BENCHMARK_RESULT) | go tool -modfile=$(GITROOT)/go.mod github.com/elastic/gobench -es $(GOBENCH_HOST) -es-username $(GOBENCH_USERNAME) -es-password $(GOBENCH_PASSWORD) \
	-index $(GOBENCH_INDEX) -tag "$(GOBENCH_DEFAULT_TAGS),$(GOBENCH_TAGS)"

.PHONY: _default-gobench-vars
_default-gobench-vars:
ifeq ($(RUN_STANDALONE),true)
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),apm_server_size=$(shell echo var.standalone_apm_server_instance_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),moxy_size=$(shell echo var.standalone_moxy_instance_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),build_sha=$(shell git rev-parse HEAD))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),bench_mode=standalone)
else
# TODO(marclop) Update code below to use a foor loop, rather than copying the lines.
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),apm_server_size=$(shell echo var.apm_server_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),elasticsearch_size=$(shell echo var.elasticsearch_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),stack_version=$(shell echo var.stack_version | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),apm_server_zone_count=$(shell echo var.apm_server_zone_count | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),elasticsearch_zone_count=$(shell echo var.elasticsearch_zone_count | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),build_sha=$(shell curl -sL -H "Authorization: Bearer $(shell terraform output -raw apm_secret_token )" $(shell terraform output -raw apm_server_url ) | jq -r '.build_sha'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),bench_mode=cloud)
endif

.PHONY: cat-apm-server-logs
cat-apm-server-logs:
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(APM_SERVER_IP) "cat /var/log/apm-server/*"

$(SSH_KEY):
	@ssh-keygen -t rsa -b 4096 -C "$(USER)@elastic.co" -N "" -f $(SSH_KEY)

.PHONY: ssh
ssh:
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(WORKER_IP)

IMAGE_TAG?=$(shell grep docker.elastic.co/kibana ${REPO_ROOT}/docker-compose.yml | cut -d: -f3)
# Tag custom images with the username and current timestamp.
# The timestamp must be included to force images to be pulled.
USER_NAME?=${USER}
CUSTOM_IMAGE_TAG:=${IMAGE_TAG}-${USER_NAME}-$(shell date +%s)
ELASTIC_AGENT_DOCKER_IMAGE?=docker.elastic.co/cloud-release/elastic-agent-cloud
ELASTICSEARCH_DOCKER_IMAGE=docker.elastic.co/cloud-release/elasticsearch-cloud-ess
KIBANA_DOCKER_IMAGE=docker.elastic.co/cloud-release/kibana-cloud
CI_ELASTIC_AGENT_DOCKER_IMAGE=docker.elastic.co/observability-ci/elastic-agent
ELASTIC_AGENT_IMAGE_TAG?=${IMAGE_TAG}

.PHONY: docker-override-committed-version
docker-override-committed-version: docker_image.auto.tfvars elastic_agent_docker_image
	@echo '-> docker image override completed'

.PHONY: cleanup-elasticsearch
cleanup-elasticsearch:
	$(eval ELASTICSEARCH_URL = $(shell terraform output elasticsearch_url))
	$(eval ELASTICSEARCH_USER = $(shell terraform output -raw elasticsearch_username))
	$(eval ELASTICSEARCH_PASS = $(shell terraform output -raw elasticsearch_password))
	$(eval APM_DATA_STREAMS = traces-apm*,metrics-apm*,logs-apm*)
	@ echo "-> Deleting APM Server data streams..."
	@ curl -u $(ELASTICSEARCH_USER):$(ELASTICSEARCH_PASS) -XDELETE $(ELASTICSEARCH_URL)/_data_stream/$(APM_DATA_STREAMS)

##############################################################################
# Target for creating a .tfvars file, defining the custom Docker images to
# use in the deployment.
##############################################################################

docker_image.auto.tfvars:
	@echo 'docker_image_override={"elasticsearch":"${ELASTICSEARCH_DOCKER_IMAGE}","kibana":"${KIBANA_DOCKER_IMAGE}","apm":"${CI_ELASTIC_AGENT_DOCKER_IMAGE}"}' > $@
	@echo 'docker_image_tag_override={"elasticsearch":"${IMAGE_TAG}","kibana":"${IMAGE_TAG}","apm":"${CUSTOM_IMAGE_TAG}"}' >> $@

##############################################################################
# Targets for building and pushing a custom Elastic Agent image.
##############################################################################

# elastic_agent_docker_image builds the Cloud Elastic Agent image
# with the local APM Server binary injected. The image will be based
# off the stack version defined in ${REPO_ROOT}/docker-compose.yml,
# unless overridden.
elastic_agent_docker_image:
	@env BASE_IMAGE=${ELASTIC_AGENT_DOCKER_IMAGE}:${ELASTIC_AGENT_IMAGE_TAG} \
		bash ${REPO_ROOT}/testing/docker/elastic-agent/build.sh \
		     -t ${CI_ELASTIC_AGENT_DOCKER_IMAGE}:${CUSTOM_IMAGE_TAG}
