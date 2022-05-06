FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make cloud-agent QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM alpine:3.15
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add iptables && \
    apk add ip6tables && \
    apk add ipset && \
    rm -rf /var/cache/apk/*

COPY --from=builder /fabedge/_output/fabedge-cloud-agent /

COPY --from=builder /fabedge/build/cloud-agent/entrypoint.sh /
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]