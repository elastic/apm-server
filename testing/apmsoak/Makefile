APMSOAK_PATH ?= ../../systemtest/cmd/apmsoak
APMSOAK_GOOS ?= linux
APMSOAK_GOARCH ?= 386

.default: all

.PHONY: all
all: terraform.tfvars init apmsoak apply

include $(shell git rev-parse --show-toplevel)/go.mk

terraform.tfvars:
	@cp terraform.tfvars.example terraform.tfvars

.PHONY: use-production
use-production:
	@terraform workspace select production || terraform worksapce new production

.PHONY: apmsoak
apmsoak:
	@echo "-> Building apmsoak..."
	@cd $(APMSOAK_PATH) && GOOS=$(APMSOAK_GOOS) GOARCH=$(APMSOAK_GOARCH) $(GO) build .

.PHONY: init
init:
	@terraform init

.PHONY: plan
plan:
	@terraform plan

.PHONY: apply
apply:
	@terraform apply -auto-approve

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve
