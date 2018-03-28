FROM golang:1.9.4
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN set -x && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
         netcat python-pip virtualenv && \
    apt-get clean

RUN pip install --upgrade setuptools

# Setup work environment
ENV APMSERVER_PATH /go/src/github.com/elastic/apm-server

RUN mkdir -p $APMSERVER_PATH/build/coverage
WORKDIR $APMSERVER_PATH

# Add healthcheck for docker/healthcheck metricset to check during testing
HEALTHCHECK CMD exit 0
