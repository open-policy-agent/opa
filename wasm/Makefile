DOCKER := docker
DOCKER_FLAGS := --rm -e DEBUG
DEBUG ?= 0

ifeq ($(shell tty > /dev/null && echo 1 || echo 0), 1)
DOCKER_FLAGS += -it
endif

DOCKER_WASM_BUILDER_IMAGE ?= openpolicyagent/opa-wasm-builder
WASM_BUILDER_VERSION := 1.6
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

CPPFLAGS += \
	-std=c++17 \
	-MD \
	-MP \
	-nodefaultlibs \
	--target=wasm32-unknown-unknown-wasm \
	-fno-exceptions \
	-fno-rtti \
	-I src/lib \
	-I src/libc++ \
	-I /usr/lib/llvm-13/include/c++/v1 \
	-I /usr/lib/llvm-13/lib/clang/13.0.0/include \
	-I src/re2 \
	-D_LIBCPP_HAS_NO_THREADS \
	-D_LIBCPP_HAS_NO_LIBRARY_ALIGNED_ALLOCATION

ifeq ($(DEBUG), 1)
CFLAGS += -O1 -gdwarf -DDEBUG
CPPFLAGS += -O1 -gdwarf -DDEBUG
else
CFLAGS += -O3
CPPFLAGS += -O3
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
	@$(DOCKER) run $(DOCKER_FLAGS) -v $(CURDIR):/src $(WASM_BUILDER_IMAGE) \
	  make --no-builtin-rules $(WASM_OBJ_DIR)/opa.wasm $(WASM_OBJ_DIR)/callgraph.csv

.PHONY: test
test:
	@$(DOCKER) run $(DOCKER_FLAGS) -v $(CURDIR):/src $(WASM_BUILDER_IMAGE) make $(WASM_OBJ_DIR)/opa-test.wasm
	@$(DOCKER) run $(DOCKER_FLAGS) -e VERBOSE -v $(CURDIR):/src -w /src node:14 node test.js $(WASM_OBJ_DIR)/opa-test.wasm

.PHONY: hack
hack:
	@$(DOCKER) run $(DOCKER_FLAGS) -v $(CURDIR):/src $(WASM_BUILDER_IMAGE)

$(shell mkdir -p $(WASM_OBJ_DIR)/src/lib)
$(shell mkdir -p $(WASM_OBJ_DIR)/src/libmpdec)
$(shell mkdir -p $(WASM_OBJ_DIR)/src/libc++)
$(shell mkdir -p $(WASM_OBJ_DIR)/src/re2/re2)
$(shell mkdir -p $(WASM_OBJ_DIR)/src/re2/util)
$(shell mkdir -p $(WASM_OBJ_DIR)/src)
$(shell mkdir -p $(WASM_OBJ_DIR)/tests)

SRCS := $(sort $(wildcard src/*.c))
CPP_SRCS := $(sort $(wildcard src/*.cc))
LIB_SRCS := $(sort $(wildcard src/lib/*.c))
LIB_MPDEC_SRCS := $(sort $(wildcard src/libmpdec/*.c))
LIB_CPP_SRCS := $(sort $(wildcard src/libc++/*.cc))
RE2_RE2_SRCS := $(sort $(wildcard src/re2/re2/*.cc))
RE2_UTIL_SRCS := $(sort $(wildcard src/re2/util/*.cc))
TEST_SRCS := $(sort $(wildcard tests/*.c))
TEST_CPP_SRCS := $(sort $(wildcard tests/*.cc))

-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(SRCS))
-include $(patsubst %.cc,$(WASM_OBJ_DIR)/%.d,$(CPP_SRCS))
-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(LIB_SRCS))
-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(LIB_MPDEC_SRCS))
-include $(patsubst %.cc,$(WASM_OBJ_DIR)/%.d,$(LIB_CPP_SRCS))
-include $(patsubst %.cc,$(WASM_OBJ_DIR)/%.d,$(RE2_RE2_SRCS))
-include $(patsubst %.cc,$(WASM_OBJ_DIR)/%.d,$(RE2_UTIL_SRCS))
-include $(patsubst %.c,$(WASM_OBJ_DIR)/%.d,$(TEST_SRCS))
-include $(patsubst %.cc,$(WASM_OBJ_DIR)/%.d,$(TEST_CPP_SRCS))

OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(SRCS))
CPP_OBJS := $(patsubst %.cc, $(WASM_OBJ_DIR)/%.wasm, $(CPP_SRCS))
LIB_OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(LIB_SRCS))
LIB_MPDEC_OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(LIB_MPDEC_SRCS))
LIB_CPP_OBJS := $(patsubst %.cc, $(WASM_OBJ_DIR)/%.wasm, $(LIB_CPP_SRCS))
RE2_RE2_OBJS := $(patsubst %.cc, $(WASM_OBJ_DIR)/%.wasm, $(RE2_RE2_SRCS))
RE2_UTIL_OBJS := $(patsubst %.cc, $(WASM_OBJ_DIR)/%.wasm, $(RE2_UTIL_SRCS))
TEST_OBJS := $(patsubst %.c, $(WASM_OBJ_DIR)/%.wasm, $(TEST_SRCS))
TEST_CPP_OBJS := $(patsubst %.cc, $(WASM_OBJ_DIR)/%.wasm, $(TEST_CPP_SRCS))

$(OBJS): $(WASM_OBJ_DIR)/src/%.wasm: src/%.c
	$(CC) $(CFLAGS) -c -o $@ $<

$(CPP_OBJS): $(WASM_OBJ_DIR)/src/%.wasm: src/%.cc
	$(CXX) $(CPPFLAGS) -c -o $@ $<

$(LIB_OBJS): $(WASM_OBJ_DIR)/src/lib/%.wasm: src/lib/%.c
	$(CC) $(CFLAGS) -c -o $@ $<

$(LIB_MPDEC_OBJS): $(WASM_OBJ_DIR)/src/libmpdec/%.wasm: src/libmpdec/%.c
	$(CC) $(CFLAGS) -c -o $@ $<

$(LIB_CPP_OBJS): $(WASM_OBJ_DIR)/src/libc++/%.wasm: src/libc++/%.cc
	$(CXX) $(CPPFLAGS) -c -o $@ $<

$(RE2_RE2_OBJS): $(WASM_OBJ_DIR)/src/re2/re2/%.wasm: src/re2/re2/%.cc
	$(CXX) $(CPPFLAGS) -c -o $@ $<

$(RE2_UTIL_OBJS): $(WASM_OBJ_DIR)/src/re2/util/%.wasm: src/re2/util/%.cc
	$(CXX) $(CPPFLAGS) -c -o $@ $<

$(TEST_OBJS): $(WASM_OBJ_DIR)/tests/%.wasm: tests/%.c
	$(CC) $(CFLAGS) -I src -c -o $@ $<

$(TEST_CPP_OBJS): $(WASM_OBJ_DIR)/tests/%.wasm: tests/%.cc
	$(CXX) $(CPPFLAGS) -I src -c -o $@ $<

$(WASM_OBJ_DIR)/opa.wasm: $(OBJS) $(CPP_OBJS) $(LIB_OBJS) $(LIB_MPDEC_OBJS) $(LIB_CPP_OBJS) $(RE2_RE2_OBJS) $(RE2_UTIL_OBJS)
	wasm-ld \
			--allow-undefined-file=src/undefined.symbols \
			--no-entry \
			--export=__heap_base \
			--stack-first \
			-o $@ $^
	@wasm2wat $(WASM_OBJ_DIR)/opa.wasm > $(WASM_OBJ_DIR)/opa.wast

$(WASM_OBJ_DIR)/opa-test.wasm: $(OBJS) $(CPP_OBJS) $(LIB_OBJS) $(LIB_MPDEC_OBJS) $(LIB_CPP_OBJS) $(RE2_RE2_OBJS) $(RE2_UTIL_OBJS) $(TEST_OBJS) $(TEST_CPP_OBJS)
	@cat src/undefined.symbols tests/undefined.symbols > _obj/undefined.symbols
	@wasm-ld \
			--allow-undefined-file=_obj/undefined.symbols \
			--no-entry \
			--stack-first \
			-o $@ $^

$(WASM_OBJ_DIR)/callgraph.csv: $(WASM_OBJ_DIR)/opa.wasm
	# NOTE: wasm-opt will output "warning: no output file specified",
	# because we're not actually optimizing the wasm, but only extract
	# information.
	build/gen-wasm-callgraph.sh $< > $@

# These are for canceling the implicit rules of GNU Make, see
# https://www.gnu.org/software/make/manual/html_node/Canceling-Rules.html#Canceling-Rules
src/libc++/mutex: src/libc++/mutex.cc
