FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make cloud-agent CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM alpine:3.13.6
COPY --from=builder /fabedge/_output/fabedge-cloud-agent /

ENTRYPOINT ["/fabedge-cloud-agent"]
