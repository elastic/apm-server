ARG ELASTIC_AGENT_IMAGE # e.g. docker.elastic.co/cloud-release/elastic-agent-cloud:8.5.0-7dbc10f8-SNAPSHOT

FROM --platform=$TARGETPLATFORM ${ELASTIC_AGENT_IMAGE}
ARG STACK_VERSION # e.g. 8.5.0-SNAPSHOT
ARG VCS_REF_SHORT # e.g. abc123
ARG TARGETARCH
ONBUILD COPY --chmod=0755 --chown=elastic-agent apm-server-linux-${TARGETARCH} ./data/elastic-agent-${VCS_REF_SHORT}/components/apm-server
