FROM golang:1.25-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" \
    -o /out/probe-exporter ./cmd/probe-exporter

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/probe-exporter /probe-exporter
USER 65534:65534
EXPOSE 9115
ENTRYPOINT ["/probe-exporter"]
CMD ["--config=/etc/probe-exporter/targets.yaml", "--listen=:9115"]