build:
	docker build --pull --rm -f "Dockerfile" -t wpxi:latest "."

clean:
	rm -rf bin/