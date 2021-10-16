FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make connector CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct

FROM alpine:3.13.6
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk --update add iptables && \
    apk --update add ipset && \
    rm -rf /var/cache/apk/*

COPY --from=builder /fabedge/_output/fabedge-connector /usr/local/bin/connector
COPY --from=builder /fabedge/build/connector/entrypoint.sh /
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]