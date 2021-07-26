FROM alpine:3.11

RUN apk --update add strongswan && \
    rm -rf /var/cache/apk/* && \
    mkdir /ipsec.d

ADD ./deploy/utils/genCert.sh /

EXPOSE 500/udp 4500/udp

CMD ["/usr/sbin/ipsec", "start", "--nofork"]
