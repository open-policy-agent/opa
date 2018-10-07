DOCKER := docker

WASM_BUILDER_REPOSITORY := openpolicyagent/opa-wasm-builder
WASM_BUILDER_VERSION := 1.0
WASM_BUILDER_IMAGE := $(WASM_BUILDER_REPOSITORY):$(WASM_BUILDER_VERSION)
WASM_OBJ_DIR := _obj

$(shell mkdir -p $(WASM_OBJ_DIR))

CFLAGS := \
	-O3 \
	-nostdinc \
	-nodefaultlibs \
	--target=wasm32-unknown-unknown-wasm

.PHONY: all
all: build test

.PHONY: clean
clean:
	rm -fr $(WASM_OBJ_DIR)

.PHONY: builder
builder: Dockerfile
	@$(DOCKER) build -t $(WASM_BUILDER_IMAGE) -f Dockerfile .

.PHONY: build
build:
	@$(DOCKER) run -it --rm -v $(CURDIR):/src $(WASM_BUILDER_IMAGE) make $(WASM_OBJ_DIR)/opa.wasm

.PHONY: test
test:
	@$(DOCKER) run -it --rm -v $(CURDIR):/src $(WASM_BUILDER_IMAGE) make $(WASM_OBJ_DIR)/opa-test.wasm
	@$(DOCKER) run -it --rm -e VERBOSE=$(VERBOSE) -v $(CURDIR):/src -w /src node:8 node test.js $(WASM_OBJ_DIR)/opa-test.wasm

.PHONY: hack
hack:
	@$(DOCKER) run -it --rm -v $(CURDIR):/src $(WASM_BUILDER_IMAGE)

$(WASM_OBJ_DIR)/malloc.wasm: src/malloc.c
	@$(CC) $(CFLAGS) -c $^ -o $@

$(WASM_OBJ_DIR)/string.wasm: src/string.c
	@$(CC) $(CFLAGS) -c $^ -o $@

$(WASM_OBJ_DIR)/json.wasm: src/json.c
	@$(CC) $(CFLAGS) -c $^ -o $@

$(WASM_OBJ_DIR)/value.wasm: src/value.c
	@$(CC) $(CFLAGS) -c $^ -o $@

$(WASM_OBJ_DIR)/opa.wasm: $(WASM_OBJ_DIR)/malloc.wasm \
		$(WASM_OBJ_DIR)/value.wasm \
		$(WASM_OBJ_DIR)/string.wasm \
		$(WASM_OBJ_DIR)/json.wasm
	@wasm-ld-8 \
			--allow-undefined-file=src/undefined.symbols \
			--import-memory \
			--no-entry \
			--export-all \
			$^ \
			-o $@
	@wasm2wat $(WASM_OBJ_DIR)/opa.wasm > $(WASM_OBJ_DIR)/opa.wast

$(WASM_OBJ_DIR)/test.wasm: tests/test.c
	@$(CC) $(CFLAGS) -I src -c $^ -o $@

$(WASM_OBJ_DIR)/opa-test.wasm: $(WASM_OBJ_DIR)/test.wasm \
		$(WASM_OBJ_DIR)/malloc.wasm \
		$(WASM_OBJ_DIR)/value.wasm \
		$(WASM_OBJ_DIR)/string.wasm \
		$(WASM_OBJ_DIR)/json.wasm
	@wasm-ld-8 \
			--allow-undefined-file=tests/undefined.symbols \
			--import-memory \
			--no-entry \
			--export-all \
			$^ \
			-o $@
