ARG BASE_IMAGE=ubuntu:22.04
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

FROM cgr.dev/chainguard/static:latest@sha256:d44809cee093b550944c1f666ff13301f92484bfdd2e53ecaac82b5b6f89647d AS builder-certs

################################################################################
# Build stage 1
# Copy prepared files from the previous stage and complete the image.
################################################################################
FROM ${BASE_IMAGE}
ARG TARGETARCH
ARG BUILD_DATE
ARG VERSION
ARG VCS_REF

# Add an init process.
ADD --chmod=0755 packaging/docker/tini-${TARGETARCH}.tar.xz /usr/bin

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
ENV PATH=/usr/share/apm-server:$PATH

# When running under Docker, we must ensure libbeat monitoring pulls cgroup
# metrics from /sys/fs/cgroup/<subsystem>/, ignoring any paths found in
# /proc/self/cgroup.
ENV LIBBEAT_MONITORING_CGROUPS_HIERARCHY_OVERRIDE=/

# Disable libbeat's strict permissions checking, which is not relevant when
# running in Docker.
ENV BEAT_STRICT_PERMS=false

COPY --chmod=0755 packaging/docker/docker-entrypoint /usr/local/bin/docker-entrypoint
COPY --chmod=0755 licenses/ELASTIC-LICENSE-2.0.txt NOTICE.txt /licenses/

# Do not explcitly set chmod, such that directory is 0755 and file is 0644
COPY --from=builder-certs /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt

# Copy files world-readable, and create the data directory world-writeable,
# to permit running the container with arbitrary UIDs and GIDs.
WORKDIR /usr/share/apm-server
COPY --chmod=0644 apm-server.yml ./apm-server.yml
COPY --chmod=0755 --from=builder /src/apm-server ./apm-server
RUN sed -i 's/127.0.0.1:8200/0.0.0.0:8200/' apm-server.yml
RUN sed -i 's/localhost:9200/elasticsearch:9200/' apm-server.yml
RUN mkdir --mode=0777 data
RUN echo 'apm-server:*:1000:1000::/usr/share/apm-server:/bin/false' >> /etc/passwd
RUN chown -R 1000:1000 /usr/share/apm-server

# remove unnecessary packages in ubuntu-based images
# use rpm to ignore ubi images
RUN rpm -v || (dpkg -P --force-depends libgssapi-krb5-2 libtirpc3 libnsl2 libkrb5-3 libk5crypto3 libkrb5support0 && \
    rm /var/log/dpkg.log)
# remove unnecessary packages in ubi-based images
# use dpkg to ignore ubuntu images
RUN dpkg --version || (rpm -v -e --nodeps libxml2)

USER apm-server
EXPOSE 8200
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint"]
