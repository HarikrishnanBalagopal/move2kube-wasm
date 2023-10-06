BIN_DIR=./bin
BIN_NAME=move2kube.wasm

.PHONY: build
build:
	mkdir -p "${BIN_DIR}"
	GOOS=wasip1 GOARCH=wasm go build -o "${BIN_DIR}/${BIN_NAME}"
	# CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -o "${BIN_DIR}/${BIN_NAME}"
	# We have to put require github.com/sirupsen/logrus v1.9.4-0.20230606125235-dd1b4c2e81af
	# in order for logrus to work. See https://github.com/HarikrishnanBalagopal/test-wasi-fs-browser/tree/main
	# CGO_ENABLED=0 tinygo build -o "${BIN_DIR}/${BIN_NAME}" -target=wasi .

.PHONY: clean
clean:
	rm -rf "${BIN_DIR}/${BIN_NAME}"

.PHONY: run
run:
	wasmer "${BIN_DIR}/${BIN_NAME}"
