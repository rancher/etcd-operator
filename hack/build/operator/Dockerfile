FROM alpine:3.5

RUN apk add --no-cache ca-certificates

ADD _output/bin/etcd-k8s-operator /usr/local/bin
ADD _output/bin/etcd-k8s-backup /usr/local/bin
ADD _output/bin/etcd-rancher-operator /usr/local/bin

ENTRYPOINT ["/usr/local/bin/etcd-k8s-operator"]
