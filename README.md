# Managed Resource Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/isLonerism/managed-resource-operator)](https://goreportcard.com/report/github.com/isLonerism/managed-resource-operator)

A Kubernetes operator for management of specific resources by regular users outside the RBAC permission scope.

## How it works

Currently there is no way to set RBAC permissions for a specific resource - the only way to allow users to create their own CRDs, for example, is to grant permissions to create any and as much CRDs as they wish. Managed Resource Operator lets you grant permissions for specific namespaces to create, edit and delete any kind of resource based on its kind, name and namespace. The operator acts as a proxy for resource management so cluster administrators could give RBAC permissions to the operator instead of the user.

## Resources

Managed Resource Operator defines the following resources:

### ManagedResource

ManagedResource is a pointer to the actual resource within the cluster. While users might not be able to view the actual resource due to insufficient RBAC permissions, they can manage their resource through this object. There are multiple ways you can create your resource using this object:

- Plain YAML:

``` yaml
apiVersion: paas.il/v1beta1
kind: ManagedResource
metadata:
  name: managedresource-test-configmap
spec:
  source:
    yaml: |
      apiVersion: v1
      data:
        data-1: value-1
        data-2: value-2
      kind: ConfigMap
      metadata:
        name: test-configmap
        namespace: default
```

- URL:

``` yaml
apiVersion: paas.il/v1beta1
kind: ManagedResource
metadata:
  name: managedresource-tests.example.com
spec:
  source:
    url: "https://raw.githubusercontent.com/isLonerism/managed-resource-operator/master/examples/objects/apiextensions_v1beta1_tests.example.com.yaml"
```

- Embedded resource:

``` yaml
apiVersion: paas.il/v1beta1
kind: ManagedResource
metadata:
  name: managedresource-test-configmap-2
spec:
  source:
    object:
      apiVersion: v1
      data:
        data-1: value-1
        data-2: value-2
      kind: ConfigMap
      metadata:
        name: test-configmap-2
        namespace: default
```

After initial creation of the resource, no matter which method was specified, the resource will use the embedded resource format. Further editing of the object can be achieved by applying the same ManagedResource with an updated URL/YAML/Object or by directly editing the ManagedResource. Upon deletion of ManagedResource, its managed object is deleted as well.

### ManagedResourceBinding

ManagedResourceBinding resides at the cluster scope and lets cluster administrators define fine grained permissions for resource creation. For example:

``` yaml
apiVersion: paas.il/v1beta1
kind: ManagedResourceBinding
metadata:
  name: managedresourcebinding-cm-crd
spec:
  objects:
  - kind: CustomResourceDefinition
    metadata:
      name: tests.example.com
  - kind: ConfigMap
    metadata:
      name: "*"
      namespace: default
  namespaces:
  - "*"
```

The following ManagedResourceBinding defines two rules:
- ANY namespace can manage a CustomResourceDefinition object called "tests.example.com"
- ANY namespace can manage ANY ConfigMap object within the "default" namespace

Any field within the ManagedResourceBinding can either be a specific value or a wildcard value.

## A word of caution

The operator effectively bypasses the RBAC permissions defined within Kubernetes. It's strongly discouraged to grant permissions for kinds such as "RoleBinding", "ClusterRoleBinding" or any other resource related to actual RBAC permissions. In addition, it's generally not recommended to set a wildcard value to 'kind' and 'namespace' fields. Permission problems are better solved using conventional RBAC permissions, only use ManagedResource as a last resort.

## Deployment

Deploying Managed Resource Operator within your cluster is pretty straightforward. Note that the operator will need to be deployed in the **managed-resource-operator-system** namespace.

#### Prerequisites

- go 1.13+
- kubectl (logged-in as cluster-admin)

### Connected deployment

This assumes your cluster is connected to the Internet.

1. Clone/Download this repository to your machine and change your current directory to the destination directory
2. Run `make deploy`

### Disconnected (Air-Gap) deployment

This assumes your cluster does not have direct connection to the Internet.

1. Clone/Download this repostory
2. `docker pull` and `docker save` the operator image (`docker.io/vladpbr/managed-resource-operator:0.1.0`) and kube-rbac-proxy image (`gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0`)
3. Transfer the repository folder and the saved images to your target network
4. Push the operator and kube-rbac-proxy images to a disconnected image registry
5. Unpack the repository on a disconnected machine logged-in to the cluster and `cd` to that directory
6. Run `make certs` - **this will create and sign the webhook certificate using your cluster's CA**
7. Substitute the placeholder environment variables within `bundle.yaml` with actual certificates and create:
``` bash
export WEBHOOK_CA_BASE64=$(cat config/webhook/certs/ca.pem.b64)
export WEBHOOK_TLS_CRT_BASE64=$(cat config/webhook/certs/tls.crt | base64 -w0)
export WEBHOOK_TLS_KEY_BASE64=$(cat config/webhook/certs/tls.key | base64 -w0)
envsubst < deploy/bundle.yaml | kubectl create -f -
```

## License

The Managed Resource Operator is released under the Apache 2.0 license. See the [LICENSE][license_file] file for details.

[license_file]:https://github.com/isLonerism/managed-resource-operator/blob/master/LICENSE