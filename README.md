# Atomix Sidecar Injector
Mutating webhook for injecting [Atomix](https://atomix.io) agent sidecar containers
into Kubernetes pods.

## Building the project
To build the project, simply run `make`:

```bash
make
```

`make` will build the sidecar injector image locally. To push the image to the
upstream repository, run `make push`:

```bash
make push
```

## Deploying the Webhook

### Create and deploy the injector configuration
The sidecar injector requires an Atomix configuration to inject into sidecar containers.
The configuration should be supplied through a `ConfigMap` and will be used to configure
the Atomix agent:

```bash
kubectl create -f deploy/configmap.yaml
```

### Create a secret for signed key/cert pair
The webhook requires a signed TLS certificate. To generate the key/cert pair, run the
`create-signed-cert.sh` script:

```bash
./deploy/create-signed-cert.sh
```

The script will create a `Secret` including the signed cert/key pair with default options.
You can optionally provide a non-default service name, secret name, and namespace
to the script:

```bash
./deploy/create-signed-cert.sh --service atomix-sidecar-injector-svc --secret atomix-sidecar-injector-certs --namespace default
```

### Deploy the webhook server
To inject Atomix agent sidecar containers, a mutating webhook is used. To deploy the
webhook server, create a `Deployment` that runs the sidecar injector webhook server,
supplying the injector configuration to the server:

```bash
kubectl create -f deploy/deployment.yaml
```

Additionally, to expose the webhook server's API, add a service exposing port `443`:

```bash
kubectl create -f deploy/service.yaml
```

### Configure webhook admission controller
Create a `MutatingWebhookConfiguration` to allow the webhook server to mutate resources
added to the Kubernetes cluster. The webhook configuration supplied in
`deploy/mutatingwebhook.yaml` must first be prepared with the created CA bundle by
running the `patch-ca-bundle.sh` script:

```bash
./deploy/patch-ca-bundle.sh
```

Once the manifest has been updated, create the `MutatingWebhookConfiguration`:

```bash
kubectl create -f deploy/mutatingwebhook.yaml
```

### Test the sidecar injector
To test the sidecar injector, deploy the example `Deployment`:

```bash
kubectl create -f deploy/example.yaml
```

The example deployment is annotated with the following annotations:
* `sidecar-injector.atomix.io/enabled`: `"true"`
* `sidecar-injector.atomix.io/cluster`: `example-atomixcluster`

The `example-atomixcluster` cluster is the example provided by the
[Atomix Operator](https://github.com/atomix/atomix-operator).

## Operation

The sidecar injector adds an [Atomix](https://atomix.io) agent sidecar container
to annotated `Pod`s. To enable Atomix sidecar injection for a `Pod`, `Deployment`,
`StatefulSet`, etc use the `sidecar-injector.atomix.io/enabled` annotation:

```yaml
annotations:
  sidecar-injector.atomix.io/enabled: "true"
```

The configuration can optionally specify the Atomix cluster to which to connect the
sidecar container:

```yaml
annotations:
  sidecar-injector.atomix.io/cluster: my-atomix-cluster
```

The sidecar injector expects to connect sidecars to Atomix clusters deployed using
the [Atomix Operator](https://github.com/atomix/atomix-operator).

The Atomix agent version can also be set through annotations:

```yaml
annotations:
  sidecar-injector.atomix.io/version: 3.1.0
```
