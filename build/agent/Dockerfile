FROM golang:1.17.13 as builder
COPY . /fabedge
RUN cd /fabedge && make agent QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM fabedge/cni-plugins:v1.4.0 as cni-plugins

FROM fabedge/base-image:0.1.0

COPY --from=builder /fabedge/build/agent/entrypoint.sh /
COPY --from=builder /fabedge/_output/fabedge-agent /usr/local/bin
COPY --from=cni-plugins /plugins/ /plugins

ENTRYPOINT ["/entrypoint.sh"]