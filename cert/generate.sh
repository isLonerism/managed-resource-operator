#!/bin/bash

# ensure certificate directory
mkdir -p config/webhook/certs
rm -rf config/webhook/certs/*

# generate request and key
openssl req -nodes -newkey rsa:2048 \
-keyout config/webhook/certs/tls.key \
-out config/webhook/certs/tls.csr \
-config ./cert/req.conf

# create k8s request
CSR_BASE64=$(cat config/webhook/certs/tls.csr | base64 -w0)
CSR_REQUEST="\
apiVersion: certificates.k8s.io/v1beta1\n\
kind: CertificateSigningRequest\n\
metadata:\n\
  name: managed-resource-webhooks\n\
spec:\n\
  groups:\n\
  - system:authenticated\n\
    request: ${CSR_BASE64}\n\
  usages:\n\
  - digital signature\n\
  - key encipherment\n\
  - server auth\n"

# sign the request
kubectl delete csr managed-resource-webhooks 2> /dev/null
echo -ne $CSR_REQUEST | kubectl create -f -
kubectl certificate approve managed-resource-webhooks

# await certificate approval
while [ "$(kubectl get csr managed-resource-webhooks -o jsonpath='{.status.certificate}' | base64 --decode | tee config/webhook/certs/tls.crt)" == "" ]
do
  sleep 1
done

# store cluster ca certificate
kubectl get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 -w0 > config/webhook/certs/ca.pem.b64