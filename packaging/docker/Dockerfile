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

FROM cgr.dev/chainguard/static:latest@sha256:9cef3c6a78264cb7e25923bf1bf7f39476dccbcc993af9f4ffeb191b77a7951e AS builder-certs

################################################################################
# Build stage 1
# Copy prepared files from the previous stage and complete the image.
################################################################################
FROM registry.access.redhat.com/ubi9/ubi-micro:latest@sha256:e9765516d74cafded50d8ef593331eeca2ef6eababdda118e5297898d99b7433
ARG TARGETARCH
ARG BUILD_DATE
ARG VERSION
ARG VCS_REF

# Statically defined labels.
LABEL \
  org.label-schema.schema-version="1.0" \
  org.label-schema.vendor="Elastic" \
  org.label-schema.license="Elastic License" \
  org.label-schema.name="apm-server" \
  org.label-schema.url="https://www.elastic.co/apm" \
  org.label-schema.vcs-url="github.com/elastic/apm-server" \
  io.k8s.description="Elastic APM Server" \
  io.k8s.display-name="Apm-Server image" \
  org.opencontainers.image.licenses="Elastic License" \
  org.opencontainers.image.title="Apm-Server" \
  org.opencontainers.image.vendor="Elastic" \
  name="apm-server" \
  maintainer="infra@elastic.co" \
  vendor="Elastic" \
  release="1" \
  url="https://www.elastic.co/apm" \
  summary="apm-server" \
  license="Elastic License" \
  description="Elastic APM Server"

# Dynamic labels, only set in published images.
LABEL \
  org.label-schema.build-date=${BUILD_DATE} \
  org.label-schema.version=${VERSION} \
  org.label-schema.vcs-ref=${VCS_REF} \
  org.opencontainers.image.created=${BUILD_DATE} \
  version=${VERSION}

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
