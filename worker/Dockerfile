FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates poppler-utils

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker-server .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata poppler-utils

WORKDIR /app

COPY --from=builder /app/worker-server .

CMD ["./worker-server"]
