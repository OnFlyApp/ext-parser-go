.PHONY: all wasm native clean

# Output binaries
WASM_BIN := bin/go-parser.wasm
NATIVE_BIN := bin/go-parser

all: wasm native

wasm:
	@echo "Building WASM plugin (wasip1/wasm)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o $(WASM_BIN) .
	@echo "WASM binary compiled successfully to $(WASM_BIN)"

native:
	@echo "Building native STDIO plugin..."
	@mkdir -p bin
	go build -o $(NATIVE_BIN) main.go
	@echo "Native binary compiled successfully to $(NATIVE_BIN)"

clean:
	@echo "Cleaning up build directory..."
	@rm -rf bin/