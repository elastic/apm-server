FROM golang:1.8.3
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN set -x && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
         netcat python-pip virtualenv && \
    apt-get clean

# Setup work environment
ENV apm-server_PATH /go/src/github.com/elastic/apm-server

RUN mkdir -p $apm-server_PATH
WORKDIR $apm-server_PATH

COPY . $apm-server_PATH

RUN make

CMD ./apm-server -e -d "*"

# Add healthcheck for docker/healthcheck metricset to check during testing
HEALTHCHECK CMD exit 0
