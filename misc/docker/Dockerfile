FROM alpine:3.21 AS runtime

RUN apk add \
    ca-certificates \
    openssl \
    gcompat \
  && update-ca-certificates

COPY calli /usr/local/bin/calli

RUN mkdir -p /etc/calli && /usr/local/bin/calli -dump-config > /etc/calli/config.yml

CMD ["/usr/local/bin/calli", "-config", "/etc/calli/config.yml"]