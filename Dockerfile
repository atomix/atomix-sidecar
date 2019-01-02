FROM alpine:latest

ADD atomix-sidecar /atomix-sidecar-injector
ENTRYPOINT ["./atomix-sidecar-injector"]