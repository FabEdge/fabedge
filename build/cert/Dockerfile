FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make cert QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM alpine:3.15
COPY --from=builder /fabedge/_output/fabedge-cert /usr/local/bin/

ENTRYPOINT ["/usr/local/bin/fabedge-cert"]
