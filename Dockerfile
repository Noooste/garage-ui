FROM node:25-alpine3.22 AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json* ./

RUN npm ci

COPY frontend/ .

RUN npm run build

FROM golang:1.25.4-alpine3.22 AS backend-builder

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY backend/go.mod backend/go.sum ./

RUN go mod download

COPY backend .

RUN swag init

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o garage-ui .

FROM alpine:3.22

WORKDIR /app

RUN apk --no-cache add ca-certificates wget

RUN addgroup -g 1000 garageui && \
    adduser -D -u 1000 -G garageui garageui

COPY --from=backend-builder --chown=garageui:garageui /app/garage-ui .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

USER garageui

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./garage-ui"]

