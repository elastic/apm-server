FROM golang:1.9.4
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN set -x && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
         netcat python-pip virtualenv && \
    apt-get clean

RUN pip install --upgrade setuptools

# Setup work environment
ENV APM_SERVER_PATH /go/src/github.com/elastic/apm-server

RUN mkdir -p $APM_SERVER_PATH
WORKDIR $APM_SERVER_PATH

COPY . $APM_SERVER_PATH

RUN make

CMD ./apm-server -e -d "*"

# Add healthcheck for docker/healthcheck metricset to check during testing
HEALTHCHECK CMD exit 0
