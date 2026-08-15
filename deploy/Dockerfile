FROM node:22-alpine AS frontend-builder

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend-builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-builder /src/frontend/dist ./webui/dist
RUN CGO_ENABLED=0 go build -buildvcs=false -tags=embed_webui -o /out/opskeeper-api ./cmd/api \
    && CGO_ENABLED=0 go build -buildvcs=false -o /out/opskeeper-worker ./cmd/worker \
    && CGO_ENABLED=0 go build -buildvcs=false -o /out/opskeeper-scheduler ./cmd/scheduler \
    && CGO_ENABLED=0 go build -buildvcs=false -o /out/opskeeper-migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=backend-builder /out/ /app/
EXPOSE 8080
ENTRYPOINT ["/app/opskeeper-api"]
