FROM golang:1.22-alpine AS builder

# Dependencias del sistema para compilar CGO (no necesario en este caso)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copiar archivos de módulo primero para aprovechar la caché de Docker
COPY go.mod go.sum ./
RUN go mod download

# Copiar el resto del código fuente
COPY . .

# Compilar el binario estático
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-server .

# ── Imagen final mínima ──────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/api-server .

EXPOSE 8080

CMD ["./api-server"]
