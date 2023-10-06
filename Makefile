BIN_DIR=./bin
BIN_NAME=move2kube.wasm

.PHONY: build
build:
	mkdir -p "${BIN_DIR}"
	GOOS=wasip1 GOARCH=wasm go build -o "${BIN_DIR}/${BIN_NAME}"
