ARG GOLANG_VERSION

################################################################################
# Build stage 0
# Build the apm-server binary. The golang image version is kept
# up to date with go.mod by Makefile.
################################################################################
FROM golang:${GOLANG_VERSION} AS builder
WORKDIR /src
COPY go.mod go.sum /src/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make apm-server

COPY --chmod=0644 apm-server.yml ./apm-server.yml
RUN sed -i 's/127.0.0.1:8200/0.0.0.0:8200/' apm-server.yml
RUN sed -i 's/localhost:9200/elasticsearch:9200/' apm-server.yml

FROM cgr.dev/chainguard/static:latest@sha256:77d8b8925dc27970ec2f48243f44c7a260d52c49cd778288e4ee97566e0cb75b AS builder-certs

################################################################################
# Build stage 1
# Copy prepared files from the previous stage and complete the image.
################################################################################
FROM registry.access.redhat.com/ubi10-micro:latest@sha256:6b19fae23a4bab50b2494471674577365a64667365d658671e3f9e8a5a698df3
ARG TARGETARCH
ARG BUILD_DATE
ARG VERSION
ARG VCS_REF

# Statically defined labels.
LABEL \
  org.opencontainers.image.authors="obs-ds-intake-services" \
  org.opencontainers.image.base.name="registry.access.redhat.com/ubi10-micro:latest" \
  org.opencontainers.image.description="Elastic APM Server" \
  org.opencontainers.image.documentation="https://www.elastic.co/docs/solutions/observability/apm/apm-server" \
  org.opencontainers.image.licenses="Elastic-2.0" \
  org.opencontainers.image.source="https://github.com/elastic/apm-server" \
  org.opencontainers.image.title="Apm-Server" \
  org.opencontainers.image.url="https://github.com/elastic/apm-server" \
  org.opencontainers.image.vendor="Elastic" \
  io.k8s.description="Elastic APM Server" \
  io.k8s.display-name="Apm-Server image"

# Dynamic labels, only set in published images.
LABEL \
  org.opencontainers.image.created=${BUILD_DATE} \
  org.opencontainers.image.revision=${VCS_REF} \
  org.opencontainers.image.version=${VERSION}

# Overwrite ubi base image non-standard labels
LABEL \
  build-date="" \
  description="" \
  maintainer="" \
  name="" \
  release="" \
  summary="" \
  url="" \
  vcs-ref="" \
  vendor="" \
  version=""

ENV ELASTIC_CONTAINER="true"

# When running under Docker, we must ensure libbeat monitoring pulls cgroup
# metrics from /sys/fs/cgroup/<subsystem>/, ignoring any paths found in
# /proc/self/cgroup.
ENV LIBBEAT_MONITORING_CGROUPS_HIERARCHY_OVERRIDE=/

# Disable libbeat's strict permissions checking, which is not relevant when
# running in Docker.
ENV BEAT_STRICT_PERMS=false

COPY --chmod=0755 licenses/ELASTIC-LICENSE-2.0.txt NOTICE.txt /licenses/
COPY --from=builder-certs /etc/pki /etc/pki
COPY --from=builder-certs /etc/ssl /etc/ssl

WORKDIR /usr/share/apm-server

RUN echo 'apm-server:*:1000:1000::/usr/share/apm-server:/bin/false' >> /etc/passwd
RUN chown -R 1000:1000 /usr/share/apm-server

COPY --chmod=0755 --chown=1000:1000 --from=builder /src/apm-server ./apm-server
COPY --chmod=0644 --chown=1000:1000 --from=builder /src/apm-server.yml ./apm-server.yml

USER apm-server
EXPOSE 8200
ENTRYPOINT ["/usr/share/apm-server/apm-server", "--environment=container"]
