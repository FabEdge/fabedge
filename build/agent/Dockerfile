FROM golang:1.16.4 as builder
COPY . /fabedge
ADD http://116.62.127.76/cni-plugins-linux-amd64-v0.9.1.tgz /tmp
RUN cd /fabedge && make agent-quick CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct && \
    tar -xzf /tmp/cni-plugins-linux-amd64-v0.9.1.tgz -C /tmp/ && \
    chmod a+x /fabedge/pkg/agent/env_prepare.sh && \
    cp /fabedge/pkg/agent/env_prepare.sh /tmp

FROM alpine:3.13.6
COPY --from=builder /fabedge/_output/fabedge-agent /tmp/bridge /tmp/host-local /tmp/loopback /tmp/env_prepare.sh /usr/local/bin/
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add iptables && \
    apk add ipvsadm && \
    apk add ipset && \
    rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/fabedge-agent"]