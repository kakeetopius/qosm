.DEFAULT_GOAL = build

build: 
	@go build . -o qosm

install:
	@go build -ldflags="-s -w" -o qosm
	@mv ./qosm /usr/local/bin

PROTO_DIR := ./internal/protobuf

PROTO_FILES := qosm.proto

PROTO_PATHS := $(addprefix $(PROTO_DIR)/, $(PROTO_FILES))

proto: $(PROTO_PATHS)
	@echo "Compiling .proto files."
	protoc --proto_path=$(PROTO_DIR) --go_out=$(PROTO_DIR) --go_opt=paths=source_relative $^



SERVICES_FILE := ./internal/service/iana_ports.go

services: | $(SERVICES_FILE)
	@go run ./internal/service/genservices.go

$(SERVICES_FILE):
	@touch $@

