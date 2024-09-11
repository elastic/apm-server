APMBENCH_PATH ?= ../../systemtest/cmd/apmbench
APMBENCH_GOOS ?= linux
APMBENCH_GOARCH ?= amd64

MOXY_PATH ?= ../../systemtest/cmd/moxy
MOXY_GOOS ?= linux
MOXY_GOARCH ?= amd64

BENCHMARK_WARMUP_TIME ?= 5m
BENCHMARK_AGENTS ?= 64
BENCHMARK_COUNT ?= 6
BENCHMARK_TIME ?= 2m
BENCHMARK_RUN ?= Benchmark
BENCHMARK_RESULT ?= benchmark-result.txt
BENCHMARK_DETAILED ?= true
BENCHMARK_EVENT_RATE ?= 0/s

GOBENCH_INDEX ?= apmbench-standalone-v2
GOBENCH_USERNAME ?= admin
GOBENCH_PASSWORD ?= changeme
GOBENCH_HOST ?= http://localhost:9200
GOBENCH_DEFAULT_TAGS = apm_server_version=$(APM_SERVER_VERSION),apm_server_type=standalone

SSH_USER ?= ec2-user
SSH_OPTS ?= -o LogLevel=ERROR -o StrictHostKeyChecking=no -o ServerAliveInterval=60 -o ServerAliveCountMax=10
SSH_KEY ?= ~/.ssh/id_rsa_terraform
WORKER_IP = $(shell terraform output -raw public_ip)

SHELL = /bin/bash
.SHELLFLAGS = -o pipefail -c

# This profile will also be used by the Terraform provider.

# export AWS_PROFILE if CI is not defined
ifeq ($(CI),)
	export AWS_PROFILE ?= default
endif


.default: all

.PHONY: all
all: $(SSH_KEY) terraform.tfvars apmbench auth apply

MAKEFILE_PATH:=$(abspath $(lastword ${MAKEFILE_LIST}))
MAKEFILE_DIR:=$(dir ${MAKEFILE_PATH})
REPO_ROOT:=$(abspath ${MAKEFILE_DIR}/../../)

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

.PHONY: apmbench
apmbench:
	@echo "-> Building apmbench..."
	@cd $(APMBENCH_PATH) && CGO_ENABLED=0 GOOS=$(APMBENCH_GOOS) GOARCH=$(APMBENCH_GOARCH) go build .

.PHONY: moxy
moxy:
	@echo "-> Building moxy..."
	@cd $(MOXY_PATH) && CGO_ENABLED=0 GOOS=$(MOXY_GOOS) GOARCH=$(MOXY_GOARCH) go build .

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
	-benchtime=$(BENCHMARK_TIME) -count=$(BENCHMARK_COUNT) -warmup-time=$(BENCHMARK_WARMUP_TIME) \
	-agents=$(BENCHMARK_AGENTS) -detailed=$(BENCHMARK_DETAILED) -event-rate=$(BENCHMARK_EVENT_RATE) -cpuprofile=$(BENCHMARK_CPU_OUT)" 2>&1 | tee $(BENCHMARK_RESULT)

.PHONY: run-benchmark-autotuned
run-benchmark-autotuned:
	$(eval APM_SERVER_SIZE:=$(shell echo var.apm_server_size | terraform console | tr -d '"'| tr -d 'g'))
	@ $(MAKE) run-benchmark BENCHMARK_AGENTS=$(shell echo $$(( $(BENCHMARK_AGENTS) * $(APM_SERVER_SIZE) )) )

.PHONY: index-benchmark-results
index-benchmark-results: _default-gobench-vars
	@cat $(BENCHMARK_RESULT) | go run -modfile=$(GITROOT)/tools/go.mod github.com/elastic/gobench -es $(GOBENCH_HOST) -es-username $(GOBENCH_USERNAME) -es-password $(GOBENCH_PASSWORD) \
	-index $(GOBENCH_INDEX) -tag "$(GOBENCH_DEFAULT_TAGS),$(GOBENCH_TAGS)"

.PHONY: _default-gobench-vars
_default-gobench-vars:
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),apm_server_size=$(shell echo var.apm_server_size | terraform console | tr -d '"'))
	$(eval GOBENCH_DEFAULT_TAGS = $(GOBENCH_DEFAULT_TAGS),build_sha=$(shell curl -sL -H "Authorization: Bearer $(shell terraform output -raw apm_secret_token )" $(shell terraform output -raw apm_server_url ) | jq -r '.build_sha'))

$(SSH_KEY):
	@ssh-keygen -t rsa -b 4096 -C "$(USER)@elastic.co" -N "" -f $(SSH_KEY)

.PHONY: ssh
ssh:
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(WORKER_IP)
