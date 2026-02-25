# s3-client — build and install

BINARY_NAME := s3-client

.PHONY: build install clean deps test

# Default target
all: build

build:
	go build -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME)"

install: build
	@mkdir -p $$(go env GOPATH)/bin && cp $(BINARY_NAME) $$(go env GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)"

clean:
	rm -f $(BINARY_NAME)
	@echo "Cleaned"

deps:
	go mod download
	go mod verify
	@echo "Dependencies ready"

test:
	go test -v ./...
