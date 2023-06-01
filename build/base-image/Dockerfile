FROM golang:1.17.13 as builder
RUN mkdir /iptables-wrapper && \
    cd /iptables-wrapper && \
    git clone https://github.com/kubernetes-sigs/iptables-wrappers.git . && \
    make build

FROM alpine:3.15
COPY --from=builder /iptables-wrapper/iptables-wrapper-installer.sh \
    /iptables-wrapper/bin/iptables-wrapper /

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk --update add iptables && \
    apk --update add ip6tables && \
    apk --update add ipset && \
    apk --update add ipvsadm && \
    rm -rf /var/cache/apk/*
