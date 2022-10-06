USER_NAME?=$(USER)
ROOT_DIR=$(shell git rev-parse --show-toplevel)
APM_SERVER_VERSION=$(shell $(MAKE) -C $(ROOT_DIR) get-version)
APM_INTEGRATION_PKG_PATH=$(ROOT_DIR)/build/packages/apm-$(APM_SERVER_VERSION).zip
RALLY_GENCORPORA_REPLAY_COUNT?=10
RALLY_BULK_CLIENTS?=10

include $(ROOT_DIR)/go.mk

.PHONY: init
init:
	@terraform init

.PHONY: plan
plan:
	@terraform plan \
		-var 'rally_workers_resource_prefix=$(USER_NAME)' \
		-var 'rally_bulk_clients=$(RALLY_BULK_CLIENTS)' \
		-var 'custom_apm_integration_pkg_path=$(APM_INTEGRATION_PKG_PATH)'

.PHONY: apply
apply: rally/corpora
	$(MAKE) -C $(ROOT_DIR) build-package
	@terraform apply -auto-approve \
		-var 'rally_workers_resource_prefix=$(USER_NAME)' \
		-var 'rally_bulk_clients=$(RALLY_BULK_CLIENTS)' \
		-var 'custom_apm_integration_pkg_path=$(APM_INTEGRATION_PKG_PATH)'

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve \
		-var 'rally_workers_resource_prefix=$(USER_NAME)' \
		-var 'rally_bulk_clients=$(RALLY_BULK_CLIENTS)' \
		-var 'custom_apm_integration_pkg_path=$(APM_INTEGRATION_PKG_PATH)'

# Generate corpora to be used in cloud testing using rally-cloud tf script
.PHONY: rally/corpora
rally/corpora:
	@rm -fr $(ROOT_DIR)/testing/rally/corpora && mkdir $(ROOT_DIR)/testing/rally/corpora
	@cd ../../systemtest/cmd/gencorpora && $(GO) run . \
		-write-dir $(ROOT_DIR)/testing/rally/corpora/ \
		-replay-count $(RALLY_GENCORPORA_REPLAY_COUNT) \
		-override-meta-source-rootdir /home/rally/rally/corpora
