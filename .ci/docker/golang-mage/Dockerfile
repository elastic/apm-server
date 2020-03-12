ARG GO_VERSION=1.12.4
FROM golang:${GO_VERSION}

WORKDIR /tools

RUN echo "Building from golang:${GO_VERSION}"
RUN git clone https://github.com/magefile/mage \
  && cd mage \
  && go run bootstrap.go \
  && mage -version \
  && cd /tools \
  && go get golang.org/x/tools/cmd/benchcmp \
  && go get github.com/t-yuki/gocover-cobertura \
  && go get github.com/jstemmer/go-junit-report \
  && go get github.com/kardianos/govendor \
  && go get github.com/elastic/go-licenser \
  && go get github.com/pierrre/gotestcover \
  && go get github.com/stretchr/testify/assert \
  && go get golang.org/x/tools/cmd/goimports

RUN apt-get update -y -qq \
  && apt-get install -y -qq  python-pip \
  && rm -rf /var/lib/apt/lists/* \
  && pip install virtualenv

WORKDIR /go
