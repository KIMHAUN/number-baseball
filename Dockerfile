FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/server /app/server
COPY web /app/web
WORKDIR /app
CMD ["/app/server"]
