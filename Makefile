all: build image
build:
	rm build/atomix-sidecar-injector
	dep ensure -v
	CGO_ENABLED=0 GOOS=linux go build -a -o build/atomix-sidecar-injector cmd/webhook/main.go
dependencies:
	dep ensure -v
image:
	docker build --no-cache -t atomix/atomix-sidecar-injector:latest build
push:
	docker push atomix/atomix-sidecar-injector:latest
.PHONY: all build dependencies image push