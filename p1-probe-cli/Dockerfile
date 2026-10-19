# ---- 构建阶段 ----
FROM golang:1.25-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download          # 依赖层单独 COPY：代码改动不会让依赖缓存失效
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w" -o /out/probe-cli ./cmd/probe-cli

# ---- 运行阶段 ----
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/probe-cli /probe-cli
USER 65534:65534
ENTRYPOINT ["/probe-cli"]
