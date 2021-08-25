FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make agent CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct && \
    wget -P /tmp -c http://116.62.127.76/cni-plugins-linux-amd64-v0.9.1.tgz && \
    tar -xf /tmp/cni-plugins-linux-amd64-v0.9.1.tgz -C /tmp/

FROM alpine:3.11
COPY --from=builder /fabedge/_output/fabedge-agent /tmp/bridge /tmp/host-local /tmp/loopback /usr/local/bin/
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add iptables && \
    apk add ipvsadm && \
    rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/fabedge-agent"]