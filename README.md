# Atomix Sidecar Ijector
Mutating webhook for injecting Atomix sidecar containers into Kubernetes pods.

## Building the project
* `dep ensure -v`
* `go build -a -o atomix-sidecar-injector`
* `docker build --no-cache -t atomix/atomix-sidecar-injector:latest`
* `docker push atomix/atomix-sidecar-injector:latest`
