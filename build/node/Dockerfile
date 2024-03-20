FROM golang:1.17.13 as builder
COPY . /fabedge
RUN cd /fabedge && make node QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM fabedge/cni-plugins:v1.4.0 as cni-plugins
FROM fabedge/base-image:0.1.0

COPY --from=builder /fabedge/_output/fabedge-node /usr/local/bin/node
COPY --from=builder /fabedge/build/node/*.sh /
COPY --from=cni-plugins /plugins/ /plugins

RUN apk --update add keepalived curl && \
    rm -rf /var/cache/apk/* && \
    chmod +x /entrypoint.sh /check-connector-leader.sh

ENTRYPOINT ["/entrypoint.sh"]