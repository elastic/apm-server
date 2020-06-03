ARG GO_VERSION=1.13.10
FROM golang:${GO_VERSION}

ENV TOOLS=/tools
WORKDIR $TOOLS
COPY go.mod .
RUN go mod download

RUN apt-get update -y -qq \
  && apt-get install -y -qq python3 python3-pip python3-venv \
  && rm -rf /var/lib/apt/lists/*
