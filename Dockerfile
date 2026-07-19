# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/snagbox ./cmd/snagbox

FROM gcr.io/distroless/static-debian12:nonroot
ARG BUILD_DATE
ARG VERSION=dev
ARG REVISION=unknown

LABEL org.opencontainers.image.source="https://github.com/supercakecrumb/snagbox" \
      org.opencontainers.image.title="snagbox" \
      org.opencontainers.image.description="Self-hosted issue intake: Telegram bot + HTTP API collecting issues into per-project queues that agents drain" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${BUILD_DATE}"

COPY --from=build /out/snagbox /snagbox

USER nonroot:nonroot

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/snagbox", "healthcheck"]

ENTRYPOINT ["/snagbox"]
