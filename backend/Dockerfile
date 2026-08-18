FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
# VERSION stamps main.buildVersion so the server only seeds skills under the
# release it was actually built from.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.buildVersion=${VERSION}" \
    -o /skillsctl-server ./backend/cmd/server

# Final image
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /skillsctl-server /skillsctl-server

USER 65534:65534
EXPOSE 8080

ENTRYPOINT ["/skillsctl-server"]
