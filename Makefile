.PHONY: fmt test build ios-build docker-build

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
