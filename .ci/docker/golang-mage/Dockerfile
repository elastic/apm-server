ARG GO_VERSION=1.13.10
FROM golang:${GO_VERSION}

ENV TOOLS=/tools
WORKDIR $TOOLS

COPY go.mod go.sum ./
COPY approvaltest/go.mod approvaltest/go.sum ./approvaltest/
COPY systemtest/go.mod systemtest/go.sum ./systemtest/
COPY internal/otel_collector/go.mod internal/otel_collector/go.sum ./internal/otel_collector/

RUN go mod download
RUN cd approvaltest && go mod download
RUN cd systemtest && go mod download
RUN cd internal/otel_collector && go mod download

RUN apt-get update -y -qq \
  && apt-get install -y -qq python3 python3-pip python3-venv \
  && rm -rf /var/lib/apt/lists/*
