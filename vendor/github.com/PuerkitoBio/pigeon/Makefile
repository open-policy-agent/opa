SHELL = /bin/bash

# directories and source code lists
ROOT = .
ROOT_SRC = $(ROOT)/*.go
BINDIR = ./bin
EXAMPLES_DIR = $(ROOT)/examples
TEST_DIR = $(ROOT)/test

# builder and ast packages
BUILDER_DIR = $(ROOT)/builder
BUILDER_SRC = $(BUILDER_DIR)/*.go
AST_DIR = $(ROOT)/ast
AST_SRC = $(AST_DIR)/*.go

# bootstrap tools variables
BOOTSTRAP_DIR = $(ROOT)/bootstrap
BOOTSTRAP_SRC = $(BOOTSTRAP_DIR)/*.go
BOOTSTRAPBUILD_DIR = $(BOOTSTRAP_DIR)/cmd/bootstrap-build
BOOTSTRAPBUILD_SRC = $(BOOTSTRAPBUILD_DIR)/*.go
BOOTSTRAPPIGEON_DIR = $(BOOTSTRAP_DIR)/cmd/bootstrap-pigeon
BOOTSTRAPPIGEON_SRC = $(BOOTSTRAPPIGEON_DIR)/*.go

# grammar variables
GRAMMAR_DIR = $(ROOT)/grammar
BOOTSTRAP_GRAMMAR = $(GRAMMAR_DIR)/bootstrap.peg
PIGEON_GRAMMAR = $(GRAMMAR_DIR)/pigeon.peg

TEST_GENERATED_SRC = $(patsubst %.peg,%.go,$(shell echo ./{examples,test}/**/*.peg))

all: $(BINDIR)/bootstrap-build $(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go \
	$(BINDIR)/bootstrap-pigeon $(ROOT)/pigeon.go $(BINDIR)/pigeon \
	$(TEST_GENERATED_SRC)

$(BINDIR)/bootstrap-build: $(BOOTSTRAPBUILD_SRC) $(BOOTSTRAP_SRC) $(BUILDER_SRC) \
	$(AST_SRC)
	go build -o $@ $(BOOTSTRAPBUILD_DIR)

$(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go: $(BINDIR)/bootstrap-build \
	$(BOOTSTRAP_GRAMMAR)
	$(BINDIR)/bootstrap-build $(BOOTSTRAP_GRAMMAR) | goimports > $@

$(BINDIR)/bootstrap-pigeon: $(BOOTSTRAPPIGEON_SRC) \
	$(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go
	go build -o $@ $(BOOTSTRAPPIGEON_DIR)

$(ROOT)/pigeon.go: $(BINDIR)/bootstrap-pigeon $(PIGEON_GRAMMAR)
	$(BINDIR)/bootstrap-pigeon $(PIGEON_GRAMMAR) | goimports > $@

$(BINDIR)/pigeon: $(ROOT_SRC) $(ROOT)/pigeon.go
	go build -o $@ $(ROOT)

$(BOOTSTRAP_GRAMMAR):
$(PIGEON_GRAMMAR):

# surely there's a better way to define the examples and test targets

$(EXAMPLES_DIR)/json/json.go: $(EXAMPLES_DIR)/json/json.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

$(EXAMPLES_DIR)/calculator/calculator.go: $(EXAMPLES_DIR)/calculator/calculator.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

$(TEST_DIR)/andnot/andnot.go: $(TEST_DIR)/andnot/andnot.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

$(TEST_DIR)/predicates/predicates.go: $(TEST_DIR)/predicates/predicates.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

$(TEST_DIR)/issue_1/issue_1.go: $(TEST_DIR)/issue_1/issue_1.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

$(TEST_DIR)/linear/linear.go: $(TEST_DIR)/linear/linear.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

$(TEST_DIR)/issue_12/issue_12.go: $(TEST_DIR)/issue_12/issue_12.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon $< | goimports > $@

lint:
	golint ./...
	go vet ./...

cmp:
	@boot=$$(mktemp) && $(BINDIR)/bootstrap-pigeon $(PIGEON_GRAMMAR) | goimports > $$boot && \
	official=$$(mktemp) && $(BINDIR)/pigeon $(PIGEON_GRAMMAR) | goimports > $$official && \
	cmp $$boot $$official && \
	unlink $$boot && \
	unlink $$official

clean:
	rm $(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go $(ROOT)/pigeon.go $(TEST_GENERATED_SRC)
	rm -rf $(BINDIR)

.PHONY: all clean lint cmp

