# Atomix Sidecar Injector
Mutating webhook for injecting Atomix agent sidecar containers into Kubernetes pods.

## Building the project
* `dep ensure -v`
* `go build -a -o atomix-sidecar-injector cmd/webhook/main.go`
* `docker build --no-cache -t atomix/atomix-sidecar-injector:latest .`
* `docker push atomix/atomix-sidecar-injector:latest`
