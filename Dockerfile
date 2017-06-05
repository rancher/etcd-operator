FROM golang:1.7-alpine

RUN apk add --update --no-cache git bash
RUN go get github.com/Masterminds/glide
ADD . /go/src/github.com/coreos/etcd-operator
WORKDIR /go/src/github.com/coreos/etcd-operator
RUN glide install --strip-vendor --strip-vcs
RUN hack/build/operator/build