FROM golang:1.17.13 as builder
COPY . /fabedge
RUN cd /fabedge && make connector QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM fabedge/base-image:0.1.0

COPY --from=builder /fabedge/_output/fabedge-connector /usr/local/bin/connector
COPY --from=builder /fabedge/build/connector/entrypoint.sh /

RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]