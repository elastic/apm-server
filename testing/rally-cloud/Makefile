GENCORPORA_PATH ?= ../../systemtest/cmd/gencorpora
GENCORPORA_GOOS ?= linux
GENCORPORA_GOARCH ?= amd64

USER_NAME?=${USER}
ROOT_DIR=$(shell git rev-parse --show-toplevel)

RALLY_GENCORPORA_REPLAY_COUNT?=10

include $(ROOT_DIR)/go.mk

# TODO: @lahsivjar Currently requires images to be built by running
# `make` on `/testing/cloud/. Explore on merging this with `testing/cloud/`
# in a way that rally_workers module can be selected as optional module.

.PHONY: init
init:
	@terraform init

.PHONY: plan
plan:
	@terraform plan \
		-var 'rally_workers_resource_prefix=$(USER_NAME)'

.PHONY: apply
apply: rally/corpora
	@terraform apply -auto-approve \
		-var 'rally_workers_resource_prefix=$(USER_NAME)'

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve \
		-var 'rally_workers_resource_prefix=$(USER_NAME)'

# Generate corpora to be used in cloud testing using rally-cloud tf script
.PHONY: rally/corpora
rally/corpora:
	@rm -fr $(ROOT_DIR)/rally/corpora && mkdir $(ROOT_DIR)/rally/corpora
	@cd ../../systemtest/cmd/gencorpora && $(GO) run . \
		-write-dir $(ROOT_DIR)/rally/corpora/ \
		-replay-count $(RALLY_GENCORPORA_REPLAY_COUNT) \
		-override-meta-source-rootdir /home/rally/rally/corpora
