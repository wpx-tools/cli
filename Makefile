build-wpci-image:
	mkdir bin
	GOOS=linux GOARCH=arm64 go build -o bin/wpci ./cmd/wpci/main.go

clean:
	rm -rf bin/

build:
	GOOS=linux GOARCH=arm64 go build -o wpx ./cmd/wpx/main.go
	docker build -t wpx/testing -f Dockerfile.test

build-wp:
	docker build -t wpx/wpdev -f Dockerfile.wp .

test: build

	docker stop wpx-docker
	docker rm wpx-docker

	docker network inspect wpx-network >/dev/null 2>&1 || \
    	docker network create --driver bridge wpx-network

	docker volume create wpx-certs-ca
	docker volume create wpx-certs-client
	docker run --privileged --name wpx-docker -d \
		--network wpx-network --network-alias docker \
		-e DOCKER_TLS_CERTDIR=/certs \
		-v wpx-certs-ca:/certs/ca \
		-v wpx-certs-client:/certs/client \
		wpx/testing