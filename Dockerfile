ARG BUILD_VERSION=canary

FROM golang:1.17-alpine3.15 as builder

RUN apk add make gcc build-base curl git
WORKDIR /firefly-perf-cli
ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN make VERSION=$BUILD_VERSION build

FROM alpine:3.15 as final
WORKDIR /firefly-perf-cli
COPY --from=builder /firefly-perf-cli/ffperf/ffperf ./ffperf
RUN ln -s /firefly-perf-cli/ffperf /usr/bin/ffperf
ENTRYPOINT ["ffperf", "run"]
CMD ["--help"]