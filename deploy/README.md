# Bundle deployment

You can use this ready `bundle.yaml` file to deploy the Managed Resource Operator for connected or disconnected environments, but you will need to generate TLS certificates for the webhooks on your own. In case of a disconnected deployment, you will also still need to transfer, tag and push the operator and kube-rbac-proxy images and change the image names within the Deployment object of `bundle.yaml`.

### Step-by-Step

1. If needed, retag and push the operator image as well as the kube-rbac-proxy image to your registry and edit the image names within `bundle.yaml` accordingly.
2. Generate a certificate signing request using the provided configuration file located at [cert/req.conf](../cert/req.conf):

``` bash
openssl req -nodes -newkey rsa:2048 -keyout tls.key -out tls.csr -config req.conf
```

3. Sign the request and store the certificate at `tls.crt` (as Base64 Encoded x.509)
4. Get the signers CA certificate and store at `ca.crt` (as Base64 Encoded x.509)
5. Substitute the placeholder environment variables within `bundle.yaml` with actual certificates and create:

``` bash
export WEBHOOK_CA_BASE64=$(cat ca.crt | base64 -w0)
export WEBHOOK_TLS_CRT_BASE64=$(cat tls.crt | base64 -w0)
export WEBHOOK_TLS_KEY_BASE64=$(cat tls.key | base64 -w0)
envsubst < bundle.yaml | kubectl create -f -
```