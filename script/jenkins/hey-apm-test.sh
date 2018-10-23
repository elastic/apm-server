set -e
export GOPATH=$WORKSPACE
export PATH=$PATH:$GOPATH/bin
eval "$(gvm 1.10.3)"
echo "Installing hey-apm dependencies and running unit tests..."
go get -v -u github.com/golang/dep/cmd/dep
go get -v -u github.com/graphaelli/hey/requester
go get -v -u github.com/olivere/elastic
go get -v -u github.com/pkg/errors
go get -v -u github.com/struCoder/pidusage
go get -v -u github.com/stretchr/testify/assert
#dep ensure -v
SKIP_EXTERNAL=1 SKIP_STRESS=1 go test -v ./...
echo "Fetching apm-server and installing latest go-licenser and mage..."
APM_SERVER_DIR=$GOPATH/src/github.com/elastic/apm-server
if [ ! -d "$APM_SERVER_DIR" ] ; then
    git clone git@github.com:elastic/apm-server.git "$APM_SERVER_DIR"
else
    (cd "$APM_SERVER_DIR" && git pull git@github.com:elastic/apm-server.git)
fi
go get -v -u github.com/elastic/go-licenser
go get -v -u github.com/magefile/mage
(cd $GOPATH/src/github.com/magefile/mage && go run bootstrap.go)
echo "Running apm-server stress tests..."
set +x
ELASTICSEARCH_URL=$CLOUD_ADDR ELASTICSEARCH_USR=$CLOUD_USERNAME ELASTICSEARCH_PWD=$CLOUD_PASSWORD go test -timeout 2h  -v github.com/elastic/hey-apm/server/client