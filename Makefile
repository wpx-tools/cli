build:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 go build -o bin/wpci ./cmd/ci/main.go
	docker build --pull --rm -f "Dockerfile" -t wpxci:latest "."

clean:
	rm -rf bin/