FROM golang:1.17.13 as builder
COPY . /fabedge
RUN cd /fabedge && make cloud-agent QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM fabedge/base-image:0.1.0

COPY --from=builder /fabedge/_output/fabedge-cloud-agent /usr/local/bin/
COPY --from=builder /fabedge/build/cloud-agent/entrypoint.sh /

RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]