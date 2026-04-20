# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS web-builder
WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
COPY api/ /src/api/
COPY docs/ /src/docs/
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25 AS go-builder
ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/tars ./cmd/tars

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=go-builder /out/tars /app/tars
COPY --from=web-builder /src/web/dist /app/web/dist

ENV TARS_WEB_DIST_DIR=/app/web/dist

EXPOSE 8081

ENTRYPOINT ["/app/tars"]
