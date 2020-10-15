DOCKER := docker

DEBUG ?= 0

DOCKER_FLAGS := --rm -e DEBUG

ifeq ($(shell tty > /dev/null && echo 1 || echo 0), 1)
DOCKER_FLAGS += -it
endif

DOCKER_WASM_BUILDER_IMAGE ?= openpolicyagent/opa-wasm-builder
WASM_BUILDER_VERSION := 1.1
WASM_BUILDER_IMAGE := $(DOCKER_WASM_BUILDER_IMAGE):$(WASM_BUILDER_VERSION)
WASM_OBJ_DIR := _obj

CFLAGS += \
	-MD \
	-MP \
	-nodefaultlibs \
	--target=wasm32-unknown-unknown-wasm \
	-I src/lib \
	-I src/libmpdec \
	-DCONFIG_32 \
	-DANSI

ifeq ($(DEBUG), 1)
CFLAGS += -O1 -gdwarf -DDEBUG
else
CFLAGS += -O3
endif

.PHONY: all
all: build test

.PHONY: clean
clean:
	rm -fr $(WASM_OBJ_DIR)

.PHONY: builder
builder: Dockerfile
	$(DOCKER) build -t $(WASM_BUILDER_IMAGE) -f Dockerfile .

.PHONY: ensure-builder
ensure-builder:
	@$(DOCKER) inspect $(WASM_BUILDER_IMAGE) > /dev/null || $(DOCKER) pull $(WASM_BUILDER_IMAGE) || $(MAKE) builder

.PHONY: push-builder
push-builder:
	$(DOCKER) pull $(WASM_BUILDER_IMAGE) || ($(MAKE) builder && $(DOCKER) push $(WASM_BUILDER_IMAGE))

.PHONY: build
build:
	@$(DOCKER) run $(DOCKER_FLAGS) -v $(CURDIR):/src $(WASM_BUILDER_IMAGE) make $(WASM_OBJ_DIR)/opa.wasm

.PHONY: test
test:
	@$(DOCKER) run $(DOCKER_FLAGS) -v $(CURDIR):/src $(WASM_BUILDER_IMAGE) make $(WASM_OBJ_DIR)/opa-test.wasm
	@$(DOCKER) run $(DOCKER_FLAGS) -e VERBOSE -v $(CURDIR):/src -w /src node:14 node test.js $(WASM_OBJ_DIR)/opa-test.wasm

.PHONY: hack
hack:
	@$(DOCKER) run $(DOCKER_FLAGS) -v $(CURDIR):/src $(WASM_BUILDER_IMAGE)

$(shell mkdir -p $(WASM_OBJ_DIR)/src/lib)
$(shell mkdir -p $(WASM_OBJ_DIR)/src/libmpdec)
$(shell mkdir -p $(WASM_OBJ_DIR)/src)
$(shell mkdir -p $(WASM_OBJ_DIR)/tests)

SRCS := $(sort $(wildcard src/*.c))
LIB_SRCS := $(sort $(wildcard src/lib/*.c))
LIB_MPDEC_SRCS := $(sort $(wildcard src/libmpdec/*.c))
TEST_SRCS := $(sort $(wildcard tests/*.c))

-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(SRCS))
-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(LIB_SRCS))
-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(LIB_MPDEC_SRCS))
-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(TEST_SRCS))

OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(SRCS))
LIB_OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(LIB_SRCS))
LIB_MPDEC_OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(LIB_MPDEC_SRCS))
TEST_OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(TEST_SRCS))

$(OBJS): $(WASM_OBJ_DIR)/src/%.wasm: src/%.c
	$(CC) $(CFLAGS) -c -o $@ $<

$(LIB_OBJS): $(WASM_OBJ_DIR)/src/lib/%.wasm: src/lib/%.c
	$(CC) $(CFLAGS) -c -o $@ $<

$(LIB_MPDEC_OBJS): $(WASM_OBJ_DIR)/src/libmpdec/%.wasm: src/libmpdec/%.c
	$(CC) $(CFLAGS) -c -o $@ $<

$(TEST_OBJS): $(WASM_OBJ_DIR)/tests/%.wasm: tests/%.c
	$(CC) $(CFLAGS) -I src -c -o $@ $<

$(WASM_OBJ_DIR)/opa.wasm: $(OBJS) $(LIB_OBJS) $(LIB_MPDEC_OBJS)
	wasm-ld-10 \
			--allow-undefined-file=src/undefined.symbols \
			--import-memory \
			--no-entry \
			--export-all \
			-o $@ $^
	@wasm2wat $(WASM_OBJ_DIR)/opa.wasm > $(WASM_OBJ_DIR)/opa.wast

$(WASM_OBJ_DIR)/opa-test.wasm: $(OBJS) $(LIB_OBJS) $(LIB_MPDEC_OBJS) $(TEST_OBJS)
	@cat src/undefined.symbols tests/undefined.symbols > _obj/undefined.symbols
	@wasm-ld-10 \
			--allow-undefined-file=_obj/undefined.symbols \
			--import-memory \
			--no-entry \
			--export-all \
			-o $@ $^
