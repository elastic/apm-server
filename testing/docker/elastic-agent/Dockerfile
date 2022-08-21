ARG ELASTIC_AGENT_IMAGE # e.g. docker.elastic.co/cloud-release/elastic-agent-cloud:8.5.0-7dbc10f8-SNAPSHOT

FROM --platform=linux/amd64 ${ELASTIC_AGENT_IMAGE} as elastic_agent_amd64
ARG STACK_VERSION # e.g. 8.5.0-SNAPSHOT
ARG VCS_REF_SHORT # e.g. abc123
ONBUILD COPY --chown=elastic-agent apm-server-linux-amd64 \
             ./data/elastic-agent-${VCS_REF_SHORT}/install/apm-server-${STACK_VERSION}-linux-x86_64/apm-server

FROM --platform=linux/arm64 ${ELASTIC_AGENT_IMAGE} as elastic_agent_arm64
ARG STACK_VERSION # e.g. 8.5.0-SNAPSHOT
ARG VCS_REF_SHORT # e.g. abc123
ONBUILD COPY --chown=elastic-agent apm-server-linux-arm64 \
             ./data/elastic-agent-${VCS_REF_SHORT}/install/apm-server-${STACK_VERSION}-linux-arm64/apm-server

FROM elastic_agent_${TARGETARCH}
