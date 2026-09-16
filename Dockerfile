FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /server ./server

# Cloud Run injects PORT at runtime; 8080 is just the local default.
ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["./server"]
