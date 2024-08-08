#!/bin/bash

set -eo pipefail

ARCH=$(uname -m)
PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
UNAME_PLATFORM=${PLATFORM}
BINARY=protoc
if [[ ${PLATFORM} == "darwin" ]]; then 
    PLATFORM=osx
    ARCH=universal_binary
elif [[ ${PLATFORM} == "linux" ]]; then
    case ${ARCH} in
    "arm64")
        ARCH=aarch_64
    ;;
    "x86_64")
        ARCH=x86_64
    ;;
    "aarch64")
    ;;
    *)
        echo "-> Architecture ${ARCH} not supported"; exit 1;
    ;;
esac
fi

PROTOBUF_VERSION="v22.1"
PROTOBUF_VERSION_NO_V=$(echo ${PROTOBUF_VERSION}|tr -d 'v')

PROTOC_PATH=build/${UNAME_PLATFORM}/${BINARY}
mkdir -p ${PROTOC_PATH}

curl -sL -o ${PROTOC_PATH}/${BINARY}.zip https://github.com/protocolbuffers/protobuf/releases/download/${PROTOBUF_VERSION}/${BINARY}-${PROTOBUF_VERSION_NO_V}-${PLATFORM}-${ARCH}.zip

BIN_PATH=${PROTOC_PATH}/bin/${BINARY}
cd ${PROTOC_PATH} && unzip ${BINARY}.zip
cd -
chmod +x ${BIN_PATH}
