ARG BUILD_FROM=ghcr.io/home-assistant/base:latest
ARG SOURCE_REPO=https://github.com/dphbfs/ha-ev-charging-bridge.git
ARG SOURCE_REF=main

FROM alpine:3.22 AS source
ARG SOURCE_REPO
ARG SOURCE_REF
RUN apk add --no-cache git
RUN git clone --depth 1 --branch "${SOURCE_REF}" "${SOURCE_REPO}" /src

FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY --from=source /src/frontend/package*.json ./
RUN npm ci
COPY --from=source /src/frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /src
RUN apk add --no-cache gcc musl-dev
COPY --from=source /src/go.mod /src/go.sum ./
RUN go mod download
COPY --from=source /src/ ./
RUN go build -o /out/ha-ev-charging-bridge .

FROM ${BUILD_FROM}
ARG BUILD_VERSION
ARG BUILD_ARCH

LABEL \
  io.hass.version="${BUILD_VERSION}" \
  io.hass.type="app" \
  io.hass.arch="${BUILD_ARCH}"

COPY --from=backend /out/ha-ev-charging-bridge /usr/local/bin/ha-ev-charging-bridge
COPY --from=frontend /src/frontend/dist /app/frontend
COPY addon/run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]
