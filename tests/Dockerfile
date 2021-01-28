FROM golang:1.15.7
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN apt-get update
RUN apt-get install -y --no-install-recommends netcat python3 python3-pip python3-venv
RUN apt-get clean

ARG UID=1000
RUN useradd -m -u $UID apm-server
USER $UID
ENV HOME=/home/apm-server
ENV GOPATH=/go

# Install the Python venv separately from the host, as otherwise the
# shebang paths will be incorrect in one or the other.
ENV PYTHON_ENV /tmp/python-env

# Install "wheel" so binary packages can be built. We end by setting
# the modification time of the "activate" script so it's considered
# out-of-date with respect to requirements.txt.
RUN python3 -m venv $PYTHON_ENV/build/ve/linux
RUN $PYTHON_ENV/build/ve/linux/bin/pip install wheel
RUN touch -d "Yesterday" $PYTHON_ENV/build/ve/linux/bin/activate

# Download module dependencies.
WORKDIR $HOME
COPY go.mod go.sum ./
COPY approvaltest/go.mod approvaltest/go.sum ./approvaltest/
COPY systemtest/go.mod systemtest/go.sum ./systemtest/
RUN go mod download
RUN cd approvaltest && go mod download
RUN cd systemtest && go mod download

# Add healthcheck for docker/healthcheck metricset to check during testing
HEALTHCHECK CMD exit 0
