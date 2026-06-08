FROM golang:1.26.4-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Pure Go — no CGO, no apk packages needed
RUN CGO_ENABLED=0 go build -o mock-vault ./testdata/mock-vault/

FROM alpine:3.23
WORKDIR /app
COPY --from=builder /build/mock-vault .
EXPOSE 8000 8001
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8000/alive || exit 1
ENTRYPOINT ["/app/mock-vault"]
