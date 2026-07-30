ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:3.21
ARG GO_BUILD_FROM=golang:1.22-alpine
FROM ${GO_BUILD_FROM} AS builder

WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=1 go build -o /out/ha-ev-charging-bridge .

FROM ${BUILD_FROM}

RUN apk add --no-cache bash busybox-extras ca-certificates jq sqlite-libs tzdata

COPY --from=builder /out/ha-ev-charging-bridge /usr/local/bin/ha-ev-charging-bridge
COPY rootfs/ /

RUN chmod +x /etc/services.d/ha-ev-charging-bridge/run

EXPOSE 8099
