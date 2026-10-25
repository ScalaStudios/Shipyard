#!/bin/sh
set -eu
d=/certs
[ -s "$d/client/key.pem" ] && exit 0
mkdir -p "$d/ca" "$d/server" "$d/client"
openssl genrsa -out "$d/ca/key.pem" 4096
openssl req -new -x509 -days 3650 -key "$d/ca/key.pem" -subj "/CN=shipyard buildkit ca" -out "$d/ca/cert.pem"
printf 'subjectAltName = DNS:buildkitd\nextendedKeyUsage = serverAuth\n' > "$d/server/ext.cnf"
printf 'extendedKeyUsage = clientAuth\n' > "$d/client/ext.cnf"
for side in server client; do
	openssl genrsa -out "$d/$side/key.pem" 4096
	openssl req -new -key "$d/$side/key.pem" -subj "/CN=buildkit-$side" -out "$d/$side/csr.pem"
	openssl x509 -req -days 3650 -in "$d/$side/csr.pem" -CA "$d/ca/cert.pem" -CAkey "$d/ca/key.pem" -CAcreateserial -extfile "$d/$side/ext.cnf" -out "$d/$side/cert.pem"
	cp "$d/ca/cert.pem" "$d/$side/ca.pem"
done
chmod 0640 "$d/client/key.pem"
