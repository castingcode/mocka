FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mockasrv ./cmd/mockasrv

FROM scratch
COPY --from=builder /app/mockasrv /mockasrv
EXPOSE 9000
ENTRYPOINT ["/mockasrv"]
