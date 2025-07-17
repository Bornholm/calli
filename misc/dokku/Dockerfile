FROM golang:1.24 AS build

RUN apt-get update && apt-get install -y make

COPY . /src

WORKDIR /src

RUN make tools/goreleaser/bin/goreleaser \
  && tools/goreleaser/bin/goreleaser build --single-target --auto-snapshot --clean

FROM alpine:3.21 AS runtime

RUN apk add \
    ca-certificates \
    openssl \
    gcompat \
  && update-ca-certificates

COPY --from=build /src/dist/calli_linux_*/calli /usr/local/bin/calli
COPY misc/dokku/Procfile /Procfile

RUN mkdir -p /etc/calli && /usr/local/bin/calli -dump-config > /etc/calli/config.yml

CMD ["/usr/local/bin/calli", "-config", "/etc/calli/config.yml"]