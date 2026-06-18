FROM golang:1.26.1-alpine AS builder
RUN echo "mocka:x:65534:65534:mocka:/:" > /etc/passwd.minimal
RUN mkdir -p /out/responses && chown 65534:65534 /out/responses
WORKDIR /app
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o mockasrv ./cmd/mockasrv

# FROM scratch
FROM alpine:latest
COPY --from=builder /etc/passwd.minimal /etc/passwd
COPY --from=builder --chown=65534:65534 /out/responses /responses
COPY --from=builder /app/mockasrv /mockasrv
USER mocka
EXPOSE 4500
ENTRYPOINT ["/mockasrv"]
