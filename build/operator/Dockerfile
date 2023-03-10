FROM golang:1.17.13 as builder
COPY . /fabedge
RUN cd /fabedge && make operator QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM alpine:3.15
COPY --from=builder /fabedge/_output/fabedge-operator /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/fabedge-operator"]