FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make operator CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM alpine:3.11
COPY --from=builder /fabedge/_output/fabedge-operator /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/fabedge-operator"]