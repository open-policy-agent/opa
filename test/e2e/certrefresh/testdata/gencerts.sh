#!/bin/bash
# adapted from
# https://github.com/dexidp/dex/blob/2d1ac74ec0ca12ae4d36072525d976c1a596820a/examples/k8s/gencert.sh#L22

cat <<EOF >req.cnf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = opa.example.com
IP.1 = 127.0.0.1
EOF

openssl genrsa -out ca-key.pem 2048
openssl req -x509 -new -nodes -key ca-key.pem -days 3650 -out ca.pem -subj "/CN=my-ca"

openssl genrsa -out server-key.pem 2048
openssl req -new -key server-key.pem -out csr.pem -subj "/CN=my-server" -config req.cnf
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -days 3650 -extensions v3_req -extfile req.cnf

openssl genrsa -out server-key-new.pem 2048
openssl req -new -key server-key-new.pem -out csr.pem -subj "/CN=my-server" -config req.cnf
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out server-cert-new.pem -days 3650 -extensions v3_req -extfile req.cnf
