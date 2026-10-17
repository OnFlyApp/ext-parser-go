.PHONY: all wasm native clean

# Output binaries
WASM_BIN := bin/go-parser.wasm
NATIVE_BIN := bin/go-parser

all: wasm native

wasm:
	@echo "Building WASM plugin (wasip1/wasm)..."
	@mkdir -p bin
	GOOS=wasip1 GOARCH=wasm go build -buildvcs=false -o $(WASM_BIN) main.go
	@echo "WASM binary compiled successfully to $(WASM_BIN)"

native:
	@echo "Building native STDIO plugin..."
	@mkdir -p bin
	go build -o $(NATIVE_BIN) main.go
	@echo "Native binary compiled successfully to $(NATIVE_BIN)"

clean:
	@echo "Cleaning up build directory..."
	@rm -rf bin/