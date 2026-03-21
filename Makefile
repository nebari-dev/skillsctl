.PHONY: proto lint test test-backend test-cli build-cli build-backend e2e clean

proto:
	cd proto && buf lint
	cd proto && buf generate

lint:
	golangci-lint run ./...

test:
	go test ./... -race -coverprofile=coverage.out

test-backend:
	go test ./backend/... -race -coverprofile=coverage.out

test-cli:
	go test ./cli/... -race -coverprofile=coverage.out

build-cli:
	CGO_ENABLED=0 go build -o skillsctl ./cli

build-backend:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o skillsctl-server ./backend/cmd/server

e2e:
	./e2e/scripts/run-e2e.sh

clean:
	rm -f skillsctl skillsctl-server coverage.out
