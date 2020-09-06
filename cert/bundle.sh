#!/bin/bash

# ensure certificate directory
mkdir -p config/webhook/certs
rm -rf config/webhook/certs/*

# set placeholder values
echo "" > config/webhook/certs/tls.crt
echo "" > config/webhook/certs/tls.key
echo "<WEBHOOK-CA-BASE64>" > config/webhook/certs/ca.pem.b64