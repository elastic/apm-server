ARG GO_VERSION=1.12.4
FROM golang:${GO_VERSION}

WORKDIR /tools

RUN echo "Building from golang:${GO_VERSION}"
RUN git clone https://github.com/magefile/mage \
  && cd mage \
  && go run bootstrap.go \
  && mage -version \
  && cd /tools \
  && go get github.com/elastic/apm-server/vendor/golang.org/x/tools/cmd/benchcmp \
  && go get github.com/elastic/apm-server/vendor/github.com/t-yuki/gocover-cobertura \
  && go get github.com/elastic/apm-server/vendor/github.com/jstemmer/go-junit-report \
  && go get github.com/elastic/apm-server/vendor/github.com/kardianos/govendor \
  && go get github.com/elastic/go-licenser \
  && go get github.com/elastic/apm-server/vendor/github.com/pierrre/gotestcover \
  && go get github.com/elastic/apm-server/vendor/github.com/stretchr/testify/assert \
  && go get github.com/elastic/apm-server/vendor/golang.org/x/tools/cmd/goimports

RUN apt-get update -y -qq \
  && apt-get install -y -qq  python3 python3-pip python3-venv \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /go
