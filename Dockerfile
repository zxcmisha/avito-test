FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /room-booking ./cmd/server

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /room-booking /app/room-booking
COPY --from=builder /app/internal/migrations /app/internal/migrations
EXPOSE 8080
CMD ["/app/room-booking"]
