# App completo numa imagem: build do React (web/dist) + API Go, que serve os dois na porta 8080.
#   docker compose --profile app up -d --build

# 1) Web
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2) API (Go puro: o leitor de PDF roda em WebAssembly, sem cgo)
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && mkdir -p /out/data

# 3) Imagem final mínima (sem shell), usuário sem privilégios
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/api /app/api
COPY --from=web /web/dist /app/web
COPY --from=api --chown=nonroot:nonroot /out/data /data
ENV IN_CONTAINER=true \
    HTTP_ADDR=0.0.0.0:8080 \
    DATA_DIR=/data \
    WEB_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/app/api"]
