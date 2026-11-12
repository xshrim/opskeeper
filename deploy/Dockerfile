FROM node:22-alpine AS frontend-builder

ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /src
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories
RUN apk add --no-cache bash make
COPY Makefile ./
COPY frontend/package.json frontend/package-lock.json frontend/
RUN cd frontend && npm ci --registry="${NPM_REGISTRY}"
COPY frontend/ frontend/
RUN make frontend-build

FROM golang:1.26-alpine AS backend-builder

ARG GOPROXY=https://goproxy.cn|https://proxy.golang.org|direct
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

ENV GOPROXY=${GOPROXY}
WORKDIR /src
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories
RUN apk add --no-cache bash make

COPY backend/go.mod backend/go.sum backend/
RUN cd backend && go mod download
COPY Makefile ./
COPY backend/ backend/
COPY --from=frontend-builder /src/frontend/dist backend/webui/dist/
RUN make backend-build VERSION="${VERSION}" COMMIT="${COMMIT}" BUILD_TIME="${BUILD_TIME}"

FROM gcr.io/distroless/static-debian12:nonroot AS runtime-base

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.revision=${COMMIT} \
      org.opencontainers.image.created=${BUILD_TIME}
EXPOSE 8080
FROM runtime-base AS api
COPY --from=backend-builder /src/backend/bin/opskeeper-api /app/opskeeper-api
ENTRYPOINT ["/app/opskeeper-api"]

FROM runtime-base AS app
COPY --from=backend-builder /src/backend/bin/opskeeper-app /app/opskeeper-app
ENTRYPOINT ["/app/opskeeper-app"]

FROM runtime-base AS worker
COPY --from=backend-builder /src/backend/bin/opskeeper-worker /app/opskeeper-worker
ENTRYPOINT ["/app/opskeeper-worker"]

FROM runtime-base AS scheduler
COPY --from=backend-builder /src/backend/bin/opskeeper-scheduler /app/opskeeper-scheduler
ENTRYPOINT ["/app/opskeeper-scheduler"]

FROM nginx:1.27-alpine AS frontend-runtime
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html
COPY deploy/nginx/frontend.conf /etc/nginx/conf.d/default.conf

FROM frontend-runtime AS frontend
