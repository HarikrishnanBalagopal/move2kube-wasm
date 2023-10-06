BIN_DIR=./bin
BIN_NAME=move2kube.wasm

.PHONY: build
build:
	mkdir -p "${BIN_DIR}"
	# GOOS=wasip1 GOARCH=wasm go build -o "${BIN_DIR}/${BIN_NAME}"
	# CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -o "${BIN_DIR}/${BIN_NAME}"
	CGO_ENABLED=0 tinygo build -o "${BIN_DIR}/${BIN_NAME}" -target=wasi .

.PHONY: clean
clean:
	rm -rf "${BIN_DIR}/${BIN_NAME}"

.PHONY: run
run:
	wasmer "${BIN_DIR}/${BIN_NAME}"
