FROM golang:1.13.8
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN apt-get update
RUN apt-get install -y --no-install-recommends netcat python3 python3-pip python3-venv
RUN apt-get clean

ARG UID=1000
RUN useradd -m -u $UID apm-server
USER $UID
ENV HOME=/home/apm-server
ENV GOPATH=$HOME/go

# Add healthcheck for docker/healthcheck metricset to check during testing
HEALTHCHECK CMD exit 0
