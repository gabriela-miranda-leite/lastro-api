FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /lastro-api ./cmd/api/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=builder /lastro-api /lastro-api

EXPOSE 8080

ENTRYPOINT ["/lastro-api"]
