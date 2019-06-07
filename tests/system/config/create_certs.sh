#!/usr/bin/env bash

set -ex

cfgdir="$1"
certdir="$2"
passphrase="$3"
if [ -z "$3" ]
  then
    exit 1
fi

rm -rf $certdir
mkdir $certdir

cakey="$certdir/ca.key.pem"
cacert="$certdir/ca.crt.pem"

## Create certificates and keys
### Generate root CA for signing certificates
openssl genrsa -out $cakey 2048
openssl req -config $cfgdir/ca.cfg -extensions extensions -key $cakey -new -x509 -out $cacert -outform pem

### Generate signed server certificate and key with passphrase
openssl genrsa -des3 -passout pass:$passphrase -out $certdir/server.key.pem 2048
openssl req -batch -new -key $certdir/server.key.pem -passin pass:$passphrase -out $certdir/server.csr -subj "/C=US/ST=SF/L=SF/O=apm/OU=apm.test/CN=localhost"
openssl x509 -req -in $certdir/server.csr -CA $cacert -CAkey $cakey -CAcreateserial -out $certdir/server.crt.pem -extfile $cfgdir/ca.cfg -extensions server
cat $cacert >> $certdir/server.crt.pem

### Generate simple client certificate and key without signature from root CA
openssl req -batch -x509 -newkey rsa:2048 -out $certdir/simple.crt.pem -keyout $certdir/simple.key.pem -nodes -days 365 -subj /CN=localhost

### Generate client certificate and key signed by the CA
openssl genrsa -out $certdir/client.key.pem 2048
openssl req -batch -new -key $certdir/client.key.pem -out $certdir/client.csr -subj "/C=US/ST=SF/L=SF/O=apm/OU=apm.test/CN=localhost"
openssl x509 -req -in $certdir/client.csr -CA $cacert -CAkey $cakey -CAcreateserial -out $certdir/client.crt.pem -extfile $cfgdir/ca.cfg -extensions client
cat $cacert >> $certdir/client.crt.pem
