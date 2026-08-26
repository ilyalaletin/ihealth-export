.PHONY: fmt test build ios-build docker-build docker-build-local

fmt:
	gofmt -w cmd/ihealth-server/main.go internal/app/*.go

test:
	go test ./...

build:
	go build ./cmd/ihealth-server

ios-build:
	xcodebuild -project ios/IHealthExporter.xcodeproj -scheme IHealthExporter -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' CODE_SIGNING_ALLOWED=NO build

docker-build:
	docker-compose build

# Avoids running the amd64 Go toolchain through QEMU on an arm64 development Mac.
docker-build-local:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o docker/ihealth-server-linux-amd64 ./cmd/ihealth-server
	docker build --platform linux/amd64 -f docker/Dockerfile.local -t ghcr.io/ilyalaletin/ihealth-export:local .
