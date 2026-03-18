FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /skillsctl-server ./backend/cmd/server

# Final image
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /skillsctl-server /skillsctl-server

USER 65534:65534
EXPOSE 8080

ENTRYPOINT ["/skillsctl-server"]
