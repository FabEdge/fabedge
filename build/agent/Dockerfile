FROM golang:1.16.4 as builder
COPY . /fabedge
RUN cd /fabedge && make agent CGO_ENABLED=0 GOPROXY=https://goproxy.io,direct

FROM alpine:3.11
COPY --from=builder /fabedge/_output/fabedge-agent /usr/local/bin/
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add iptables && \
    apk add ipvsadm && \
    rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/fabedge-agent"]