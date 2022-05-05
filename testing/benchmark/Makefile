APMBENCH_PATH ?= ../../systemtest/cmd/apmbench
APMBENCH_GOOS ?= linux
APMBENCH_GOARCH ?= amd64

BENCHMARK_WARMUP ?= 50000
BENCHMARK_AGENTS ?= 64
BENCHMARK_COUNT ?= 2
BENCHMARK_TIME ?= 5m
BENCHMARK_RUN ?= Benchmark
BENCHMARK_RESULT ?= benchmark-result.txt

GOBENCH_INDEX ?= gobench
GOBENCH_USERNAME ?= admin
GOBENCH_PASSWORD ?= changeme
GOBENCH_HOST ?= http://localhost:9200
GOBENCH_DEFAULT_TAGS = "apm_server_version=$(APM_SERVER_VERSION)"

SSH_USER ?= ec2-user
SSH_OPTS ?= -o LogLevel=ERROR -o StrictHostKeyChecking=no
SSH_KEY ?= ~/.ssh/id_rsa_terraform
WORKER_IP = $(shell terraform output -raw public_ip)

# This profile will also be used by the Terraform provider.
export AWS_PROFILE ?= default

.default: all

.PHONY: all
all: $(SSH_KEY) terraform.tfvars apmbench auth apply

include $(shell git rev-parse --show-toplevel)/go.make

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
	@sed "s/USER/$(USER)/" terraform.tfvars.example > terraform.tfvars

.PHONY: apmbench
apmbench:
	@echo "-> Building apmbench..."
	@cd $(APMBENCH_PATH) && GOOS=$(APMBENCH_GOOS) GOARCH=$(APMBENCH_GOARCH) $(GO) build .

.PHONY: init
init:
	@terraform init

.PHONY: apply
apply:
	@terraform apply -auto-approve

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve

.PHONY: run-benchmark
run-benchmark:
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(WORKER_IP) ". .envrc && bin/apmbench -run=$(BENCHMARK_RUN) \
	-benchtime=$(BENCHMARK_TIME) -count=$(BENCHMARK_COUNT) -warmup-events=$(BENCHMARK_WARMUP) \
	-agents=$(BENCHMARK_AGENTS)" 2>&1 | tee $(BENCHMARK_RESULT)

.PHONY: index-benchmark-results
index-benchmark-results: $(GOBENCH)
	@cat $(BENCHMARK_RESULT) | $(GOBENCH) -es $(GOBENCH_HOST) -es-username $(GOBENCH_USERNAME) -es-password $(GOBENCH_PASSWORD) \
	-index $(GOBENCH_INDEX) -tag "$(GOBENCH_DEFAULT_TAGS),$(GOBENCH_TAGS)"

$(SSH_KEY):
	ssh-keygen -t rsa -b 4096 -C "$(USER)@elastic.co" -N "" -f $(SSH_KEY)

.PHONY: ssh
ssh:
	@ssh $(SSH_OPTS) -i $(SSH_KEY) $(SSH_USER)@$(WORKER_IP)

.PHONY: docker-override-committed-version
docker-override-committed-version:
	$(eval TAG := $(shell grep docker.elastic.co/ ../../docker-compose.yml |grep SNAPSHOT| cut -d ':' -f3|uniq))
	@echo 'export TF_VAR_docker_image_tag_override={"elasticsearch":"$(TAG)","kibana":"$(TAG)","apm":"$(TAG)"}'
