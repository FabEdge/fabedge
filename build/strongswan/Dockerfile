FROM alpine:3.15

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && \
    apk add strongswan=5.9.1-r1 && \
    rm -rf /var/cache/apk/*

RUN sed -i 's/# install_routes = yes/install_routes = no/' /etc/strongswan.d/charon.conf

EXPOSE 500/udp 4500/udp

CMD ["/usr/sbin/ipsec", "start", "--nofork"]
