#!/bin/bash
# taken from
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
openssl req -x509 -new -nodes -key ca-key.pem -days 1000 -out ca.pem -subj "/CN=my-ca"

openssl genrsa -out client-key.pem 2048
openssl req -new -key client-key.pem -out csr.pem -subj "/CN=my-client"
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem -days 1000

openssl genrsa -out client-key-2.pem 2048
openssl req -new -key client-key-2.pem -out csr.pem -subj "/CN=my-client-2"
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out client-cert-2.pem -days 1000

openssl genrsa -out server-key.pem 2048
openssl req -new -key server-key.pem -out csr.pem -subj "/CN=my-server" -config req.cnf
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -days 1000 -extensions v3_req -extfile req.cnf
