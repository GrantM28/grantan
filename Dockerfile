FROM node:20-alpine AS ui-builder

WORKDIR /app/ui
COPY ui/package.json ./
COPY ui/next.config.js ./
COPY ui/tsconfig.json ./
COPY ui/next-env.d.ts ./
RUN npm install --no-fund --no-audit

COPY ui ./
RUN npm run build

FROM golang:1.22-alpine AS go-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=ui-builder /app/ui/out ./public
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/grantan ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN adduser -D -u 10001 grantan && mkdir -p /data/games /app/public && chown -R grantan:grantan /app /data

COPY --from=go-builder /app/grantan /app/grantan
COPY --from=go-builder /app/public /app/public

ENV PORT=5678
ENV DATA_DIR=/data
ENV STATIC_DIR=/app/public

USER grantan
EXPOSE 5678

CMD ["/app/grantan"]
