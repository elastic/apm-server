#!/usr/bin/env bash

set -ex

cfgdir="$1"
certdir="$2"
mkdir $certdir
cakey="$certdir/ca.key.pem"
cacert="$certdir/ca.crt.pem"

## remove existing certificates and keys
rm -f $certdir/*.pem $certdir/*.csr $certdir/*.srl

## Create certificates and keys

### Simple certificate and key without passphrase
openssl req -batch -x509 -newkey rsa:2048 -out $certdir/simple.crt.pem -keyout  $certdir/simple.key.pem -nodes -days 365 -subj /CN=localhost

### Cert with key passphrase is not supported by python requests library atm, see http://docs.python-requests.org/en/master/user/advanced/#client-side-certificates

### Generate root CA for signing certificates for mutual authentication
#### Create the root key
openssl genrsa -out $cakey 2048
#### Create the root certificate
openssl req -config $cfgdir/ca.cfg -extensions extensions -key $cakey -new -x509 -out $cacert -outform pem

#### Generate signed server certificate and key
openssl genrsa -out $certdir/server.key.pem 2048
openssl req -batch -new -key $certdir/server.key.pem -out $certdir/server.csr -subj "/C=US/ST=SF/L=SF/O=apm/OU=apm.test/CN=localhost"
openssl x509 -req -in $certdir/server.csr -CA $cacert -CAkey $cakey -CAcreateserial -out $certdir/server.crt.pem -extfile $cfgdir/ca.cfg -extensions server
cat $cacert >> $certdir/server.crt.pem

#### Generate signed client certificate and key
openssl genrsa -out $certdir/client.key.pem 2048
openssl req -batch -new -key $certdir/client.key.pem -out $certdir/client.csr -subj "/C=US/ST=SF/L=SF/O=apm/OU=apm.test/CN=localhost"
openssl x509 -req -in $certdir/client.csr -CA $cacert -CAkey $cakey -CAcreateserial -out $certdir/client.crt.pem -extfile $cfgdir/ca.cfg -extensions client
cat $cacert >> $certdir/client.crt.pem
