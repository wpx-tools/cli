VERSION="0.0.1"

image:
	docker build --pull --rm -f "Dockerfile" -t wpxi:latest "."

build: clean image

	docker build --pull --rm -f "Dockerfile" -t wpxi:latest "."
	
	GOOS=windows GOARCH=386 go build -o bin/windows/386/wpx ./cmd/cli/main.go
	GOOS=windows GOARCH=amd64 go build -o bin/windows/amd64/wpx ./cmd/cli/main.go
	GOOS=windows GOARCH=arm go build -o bin/windows/arm/wpx ./cmd/cli/main.go
	GOOS=windows GOARCH=arm64 go build -o bin/windows/arm64/wpx ./cmd/cli/main.go

	zip -q bin/wpx-$(VERSION)-windows-386.zip bin/windows/386/wpx
	zip -q bin/wpx-$(VERSION)-windows-amd64.zip bin/windows/amd64/wpx
	zip -q bin/wpx-$(VERSION)-windows-arm.zip bin/windows/arm/wpx
	zip -q bin/wpx-$(VERSION)-windows-arm64.zip bin/windows/arm64/wpx

	rm -rf bin/windows

	GOOS=linux GOARCH=386 go build -o bin/linux/386/wpx ./cmd/cli/main.go
	GOOS=linux GOARCH=arm go build -o bin/linux/arm/wpx ./cmd/cli/main.go
	GOOS=linux GOARCH=arm64 go build -o bin/linux/arm64/wpx ./cmd/cli/main.go
	GOOS=linux GOARCH=amd64 go build -o bin/linux/amd64/wpx ./cmd/cli/main.go

	zip -q bin/wpx-$(VERSION)-linux-386.zip bin/linux/386/wpx
	zip -q bin/wpx-$(VERSION)-linux-arm.zip bin/linux/arm/wpx
	zip -q bin/wpx-$(VERSION)-linux-arm64.zip bin/linux/arm64/wpx
	zip -q bin/wpx-$(VERSION)-linux-amd64.zip bin/linux/amd64/wpx

	rm -rf bin/linux

	GOOS=darwin GOARCH=arm64 go build -o bin/darwin/arm64/wpx ./cmd/cli/main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/darwin/amd64/wpx ./cmd/cli/main.go

	zip -q bin/wpx-$(VERSION)-darwin-arm64.zip bin/darwin/arm64/wpx
	zip -q bin/wpx-$(VERSION)-darwin-amd64.zip bin/darwin/amd64/wpx

	rm -rf bin/darwin

clean:
	rm -rf bin/