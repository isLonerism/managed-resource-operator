# Managed Resource Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/vlad-pbr/managed-resource-operator)](https://goreportcard.com/report/github.com/vlad-pbr/managed-resource-operator)

A Kubernetes operator for management of specific resources by regular users outside the RBAC permission scope.

## How it works

Currently there is no way to set RBAC permissions for a specific resource - the only way to allow users to create their own CRDs, for example, is to grant permissions to create any and as many CRDs as they wish. Managed Resource Operator lets you grant permissions for specific namespaces to create, edit and delete any kind of resource based on its kind, name and namespace. The operator acts as a proxy for resource management so cluster administrators could give RBAC permissions to the operator instead of the user.

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
    url: "https://raw.githubusercontent.com/vlad-pbr/managed-resource-operator/1.1.0/examples/objects/apiextensions_v1beta1_tests.example.com.yaml"
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

#### Overwrite field

In addition, `.spec.overwrite` field may be useful when planning your Continuous Deployment strategy. Data defined within this field will directly overwrite the fields of the resource specified by `.spec.source` field. This might help you in the following scenarios:

- You need to add additional fields to your resource based on the strategy
- You want to make sure the resource always conforms to a standard you defined
- You want to ensure your resource's fields always stay the same (like `.metadata.name` or `.metadata.namespace`)

Here is an example of an overwrite:

``` yaml
apiVersion: paas.il/v1beta1
kind: ManagedResource
metadata:
  name: managedresource-cm-overwrite-example
spec:
  source:
    url: "https://raw.githubusercontent.com/vlad-pbr/managed-resource-operator/1.1.0/examples/objects/v1_random-configmap.yaml"
  overwrite:
    metadata:
      name: overwritten-configmap-name
      namespace: default
```

This overwrite ensures that both `.metadata.name` and `.metadata.namespace` fields of a resource retrieved from the URL are '__overwritten-configmap-name__' and '__default__' respectively, even if these fields were not previously defined. Once the object is created, the overwrite will be applied and then removed from the object.

### ManagedResourceBinding

ManagedResourceBinding resides at the cluster scope and lets cluster administrators define fine grained permissions for resource creation. For example:

``` yaml
apiVersion: paas.il/v1beta1
kind: ManagedResourceBinding
metadata:
  name: managedresourcebinding-cm-crd
spec:
  items:
  - object:
      kind: CustomResourceDefinition
      metadata:
        name: tests.example.com
    verbs:
    - create
  - object:
      kind: ConfigMap
      metadata:
        name: "*"
        namespace: default
    verbs:
    - create
    - delete
  namespaces:
  - "*"
```

The following ManagedResourceBinding defines two rules:
- ANY namespace can CREATE a CustomResourceDefinition object called "tests.example.com"
- ANY namespace can CREATE and DELETE ANY ConfigMap object within the "default" namespace

Any field within the 'object' field as well as the 'namespaces' field can either be a specific value or a wildcard value.

## Configuration

Operator can be configured using the following environment variables:

- **RECONCILIATION_INTERVAL_MS**: (int) reconciliation interval (in milliseconds) for the operator
- **HTTP_INSECURE**: (bool) allow insecure server connections when using the URL source type
- **HTTP_TIMEOUT**: (int) timeout (in seconds) of a request when using the URL source type
- **HTTP_CA_BUNDLE_PATH**: (string) path to a local certificate bundle to trust when using the URL source type (use a configmap to map your bundle to the pod)

## A word of caution

The operator effectively bypasses the RBAC permissions defined within Kubernetes. It's strongly discouraged to grant permissions for kinds such as "RoleBinding", "ClusterRoleBinding" or any other resource related to actual RBAC permissions. In addition, it's generally not recommended to set a wildcard value to 'kind' and 'namespace' fields. Permission problems are better solved using conventional RBAC permissions, only use ManagedResource as a last resort.

## Deployment

Deploying Managed Resource Operator within your cluster is pretty straightforward. Note:
- The operator will need to be deployed in the **managed-resource-operator-system** namespace
- `make deploy` and `make certs` commands will attempt to **create and sign the webhook certificate using your cluster's CA**
  - You can prefix these commands with `SELF_SIGNED=true` in order to generate a self signed certificate instead

#### Prerequisites

- go 1.13+ (connected deployment only)
- kubectl (logged-in as cluster-admin)

### Connected deployment

This assumes your cluster is connected to the Internet.

1. Clone/Download this repository to your machine and change your current directory to the destination directory
2. Run `make deploy`

### Disconnected (Air-Gap) deployment

This assumes your cluster does not have direct connection to the Internet.

1. Clone/Download this repostory
2. `docker pull` and `docker save` the operator image (`docker.io/vladpbr/managed-resource-operator:1.1.0`) and kube-rbac-proxy image (`gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0`)
3. Transfer the repository folder and the saved images to your target network
4. Push the operator and kube-rbac-proxy images to a disconnected image registry
5. Unpack the repository on a disconnected machine logged-in to the cluster and `cd` to that directory
6. Run `make certs`
7. Substitute the placeholder environment variables within `bundle.yaml` with actual certificates and create:
``` bash
export WEBHOOK_CA_BASE64=$(cat config/webhook/certs/ca.pem.b64)
export WEBHOOK_TLS_CRT_BASE64=$(cat config/webhook/certs/tls.crt | base64 -w0)
export WEBHOOK_TLS_KEY_BASE64=$(cat config/webhook/certs/tls.key | base64 -w0)
envsubst < deploy/bundle.yaml | kubectl create -f -
```

## License

The Managed Resource Operator is released under the Apache 2.0 license. See the [LICENSE][license_file] file for details.

[license_file]:https://github.com/vlad-pbr/managed-resource-operator/blob/1.1.0/LICENSE