MAKEFILE_PATH:=$(abspath $(lastword ${MAKEFILE_LIST}))
MAKEFILE_DIR:=$(dir ${MAKEFILE_PATH})
REPO_ROOT:=$(abspath ${MAKEFILE_DIR}/../../)
IMAGE_TAG?=$(shell grep docker.elastic.co/kibana ${REPO_ROOT}/docker-compose.yml | cut -d: -f3)
ELASTIC_AGENT_IMAGE_TAG?=${IMAGE_TAG}

export DOCKER_BUILDKIT=1

ELASTIC_AGENT_DOCKER_IMAGE?=docker.elastic.co/cloud-release/elastic-agent-cloud
ELASTICSEARCH_DOCKER_IMAGE=docker.elastic.co/cloud-release/elasticsearch-cloud-ess
KIBANA_DOCKER_IMAGE=docker.elastic.co/cloud-release/kibana-cloud

CI_ELASTIC_AGENT_DOCKER_IMAGE=docker.elastic.co/observability-ci/elastic-agent

# Tag custom images with the username and current timestamp.
# The timestamp must be included to force images to be pulled.
USER_NAME?=${USER}
CUSTOM_IMAGE_TAG:=${IMAGE_TAG}-${USER_NAME}-$(shell date +%s)

USER_ELASTIC_AGENT_DOCKER_IMAGE=docker.elastic.co/observability-ci/${USER_NAME}-elastic-agent
USER_KIBANA_DOCKER_IMAGE=docker.elastic.co/observability-ci/${USER_NAME}-kibana

.DEFAULT_GOAL := docker_image.auto.tfvars

##############################################################################
# Target for creating a .tfvars file, defining the custom Docker images to
# use in the deployment.
##############################################################################

docker_image.auto.tfvars: elastic_agent_docker_image
	@echo 'docker_image_override={"elasticsearch":"${ELASTICSEARCH_DOCKER_IMAGE}","kibana":"${KIBANA_DOCKER_IMAGE}","apm":"${CI_ELASTIC_AGENT_DOCKER_IMAGE}"}' > $@
	@echo 'docker_image_tag_override={"elasticsearch":"${IMAGE_TAG}","kibana":"${IMAGE_TAG}","apm":"${CUSTOM_IMAGE_TAG}"}' >> $@

##############################################################################
# Terraform shortcut rules.
##############################################################################

.PHONY: apply
apply:
	@terraform apply -auto-approve

.PHONY: destroy
destroy:
	@terraform destroy -auto-approve

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
