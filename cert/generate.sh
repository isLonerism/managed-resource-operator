#!/bin/bash

# ensure certificate directory
mkdir -p /tmp/k8s-webhook-server/serving-certs
rm -rf /tmp/k8s-webhook-server/serving-certs/*

# generate request and key
openssl req -nodes -newkey rsa:2048 \
-keyout /tmp/k8s-webhook-server/serving-certs/tls.key \
-out /tmp/k8s-webhook-server/serving-certs/tls.csr \
-config ./cert/req.conf

# create k8s request
CSR_BASE64=$(cat /tmp/k8s-webhook-server/serving-certs/tls.csr | base64 -w0)
CSR_REQUEST="\
apiVersion: certificates.k8s.io/v1beta1\n\
kind: CertificateSigningRequest\n\
metadata:\n\
  name: managed-resource-operator-webhook-service.managed-resource-operator-system\n\
spec:\n\
  groups:\n\
  - system:authenticated\n\
    request: ${CSR_BASE64}\n\
  usages:\n\
  - digital signature\n\
  - key encipherment\n\
  - server auth\n"

# sign the request
kubectl delete csr managed-resource-operator-webhook-service.managed-resource-operator-system
echo -ne $CSR_REQUEST | kubectl create -f -
kubectl certificate approve managed-resource-operator-webhook-service.managed-resource-operator-system

# write signed certificate to certificate directory
kubectl get csr managed-resource-operator-webhook-service.managed-resource-operator-system -o jsonpath='{.status.certificate}' | \
base64 --decode > /tmp/k8s-webhook-server/serving-certs/tls.crt

# create secret for deployment in cluster (TODO add to kustomize)
kubectl create secret tls webhook-server-cert \
--key=/tmp/k8s-webhook-server/serving-certs/tls.key \
--cert=/tmp/k8s-webhook-server/serving-certs/tls.crt \
--dry-run -o yaml | kubectl -n managed-resource-operator-system apply -f -