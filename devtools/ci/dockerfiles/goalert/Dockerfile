# Compile with the patched SDK while retaining the existing build tools.
FROM docker.io/library/golang:1.26.10 AS security-toolchain
FROM docker.io/goalert/build-env:go1.26.3 AS build
COPY --from=security-toolchain /usr/local/go /opt/security-go
ENV GOROOT=/opt/security-go
ENV PATH="/opt/security-go/bin:${PATH}"
COPY / /build/
WORKDIR /build
RUN make clean bin/build/goalert-linux-amd64

FROM docker.io/library/alpine:3.24
RUN apk --no-cache upgrade && apk --no-cache add ca-certificates
ENV GOALERT_LISTEN=:8081
EXPOSE 8081
CMD ["/usr/bin/goalert"]

COPY --from=build /build/bin/build/goalert-linux-amd64/goalert/bin/* /usr/bin/
RUN /usr/bin/goalert self-test

