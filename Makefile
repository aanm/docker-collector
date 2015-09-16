.PHONY: tests docker-collector-production docker-collector clean docker-image


KERNEL:= $(shell uname -s)
MACHINE := $(shell uname -m)

tests: docker-collector
	@godep go test ./...

docker-collector-production: clean tests
	@godep go fmt ./...
	@godep go build -a -o docker-collector-${KERNEL}-${MACHINE} docker-collector.go containersregistry.go

docker-collector:
	@godep go fmt ./...
	@godep go build -o docker-collector-${KERNEL}-${MACHINE} docker-collector.go containersregistry.go

clean:
	@godep go clean -i
	@rm docker-collector-${KERNEL}-${MACHINE}

collector-image:
	@./scripts/build-collector-dev-image.sh

update-godeps:
	@./scripts/update-godeps.sh

all: tests
