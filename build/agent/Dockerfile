FROM golang:1.16.4 as builder
ARG pluginVersion=v0.9.1
COPY . /fabedge
RUN cd /tmp && git clone https://gitee.com/fabedge/plugins.git && \
    cd /tmp/plugins && git checkout ${pluginVersion} && bash build_linux.sh -ldflags "-extldflags -static -X 'github.com/containernetworking/plugins/pkg/utils/buildversion.BuildVersion=${pluginVersion}'" && \
    cd /tmp/plugins/bin && cp bridge host-local loopback portmap bandwidth /tmp/ && \
    cd /fabedge && make agent QUICK=1 CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct && \
    chmod a+x /fabedge/pkg/agent/env_prepare.sh && \
    cp /fabedge/pkg/agent/env_prepare.sh /tmp

FROM alpine:3.15
COPY --from=builder /fabedge/_output/fabedge-agent /tmp/bridge /tmp/host-local /tmp/loopback /tmp/portmap /tmp/bandwidth /tmp/env_prepare.sh /usr/local/bin/
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add iptables && \
    apk add ip6tables && \
    apk add ipvsadm && \
    apk add ipset && \
    rm -rf /var/cache/apk/*
ENTRYPOINT ["/usr/local/bin/fabedge-agent"]