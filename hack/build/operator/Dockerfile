FROM alpine:3.5

RUN apk add --no-cache ca-certificates

ADD _output/bin/etcd-operator /usr/local/bin

ENTRYPOINT ["/usr/local/bin/etcd-operator"]
CMD ["kubernetes", "operator"]