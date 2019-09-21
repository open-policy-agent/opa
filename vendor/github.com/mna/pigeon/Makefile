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
STATICCODEGENERATOR_DIR = $(BOOTSTRAP_DIR)/cmd/static_code_generator
STATICCODEGENERATOR_SRC = $(STATICCODEGENERATOR_DIR)/*.go

# grammar variables
GRAMMAR_DIR = $(ROOT)/grammar
BOOTSTRAP_GRAMMAR = $(GRAMMAR_DIR)/bootstrap.peg
PIGEON_GRAMMAR = $(GRAMMAR_DIR)/pigeon.peg

TEST_GENERATED_SRC = $(patsubst %.peg,%.go,$(shell echo ./{examples,test}/**/*.peg))

all: $(BUILDER_DIR)/generated_static_code.go $(BINDIR)/static_code_generator \
	$(BUILDER_DIR)/generated_static_code_range_table.go \
	$(BINDIR)/bootstrap-build $(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go \
	$(BINDIR)/bootstrap-pigeon $(ROOT)/pigeon.go $(BINDIR)/pigeon \
	$(TEST_GENERATED_SRC)

$(BINDIR)/static_code_generator: $(STATICCODEGENERATOR_SRC)
	go build -o $@ $(STATICCODEGENERATOR_DIR)

$(BINDIR)/bootstrap-build: $(BOOTSTRAPBUILD_SRC) $(BOOTSTRAP_SRC) $(BUILDER_SRC) \
	$(AST_SRC)
	go build -o $@ $(BOOTSTRAPBUILD_DIR)

$(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go: $(BINDIR)/bootstrap-build \
	$(BOOTSTRAP_GRAMMAR)
	$(BINDIR)/bootstrap-build $(BOOTSTRAP_GRAMMAR) > $@

$(BINDIR)/bootstrap-pigeon: $(BOOTSTRAPPIGEON_SRC) \
	$(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go
	go build -o $@ $(BOOTSTRAPPIGEON_DIR)

$(ROOT)/pigeon.go: $(BINDIR)/bootstrap-pigeon $(PIGEON_GRAMMAR)
	$(BINDIR)/bootstrap-pigeon $(PIGEON_GRAMMAR) > $@

$(BINDIR)/pigeon: $(ROOT_SRC) $(ROOT)/pigeon.go
	go build -o $@ $(ROOT)

$(BUILDER_DIR)/generated_static_code.go: $(BUILDER_DIR)/static_code.go $(BINDIR)/static_code_generator
	$(BINDIR)/static_code_generator $(BUILDER_DIR)/static_code.go $@ staticCode

$(BUILDER_DIR)/generated_static_code_range_table.go: $(BUILDER_DIR)/static_code_range_table.go $(BINDIR)/static_code_generator
	$(BINDIR)/static_code_generator $(BUILDER_DIR)/static_code_range_table.go $@ rangeTable0

$(BOOTSTRAP_GRAMMAR):
$(PIGEON_GRAMMAR):

# surely there's a better way to define the examples and test targets
$(EXAMPLES_DIR)/json/json.go: $(EXAMPLES_DIR)/json/json.peg $(EXAMPLES_DIR)/json/optimized/json.go $(EXAMPLES_DIR)/json/optimized-grammar/json.go $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(EXAMPLES_DIR)/json/optimized/json.go: $(EXAMPLES_DIR)/json/json.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-parser -optimize-basic-latin $< > $@

$(EXAMPLES_DIR)/json/optimized-grammar/json.go: $(EXAMPLES_DIR)/json/json.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-grammar $< > $@

$(EXAMPLES_DIR)/calculator/calculator.go: $(EXAMPLES_DIR)/calculator/calculator.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(EXAMPLES_DIR)/indentation/indentation.go: $(EXAMPLES_DIR)/indentation/indentation.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/andnot/andnot.go: $(TEST_DIR)/andnot/andnot.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/predicates/predicates.go: $(TEST_DIR)/predicates/predicates.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/issue_1/issue_1.go: $(TEST_DIR)/issue_1/issue_1.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/linear/linear.go: $(TEST_DIR)/linear/linear.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/issue_18/issue_18.go: $(TEST_DIR)/issue_18/issue_18.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/runeerror/runeerror.go: $(TEST_DIR)/runeerror/runeerror.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/errorpos/errorpos.go: $(TEST_DIR)/errorpos/errorpos.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/global_store/global_store.go: $(TEST_DIR)/global_store/global_store.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/goto/goto.go: $(TEST_DIR)/goto/goto.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/goto_state/goto_state.go: $(TEST_DIR)/goto_state/goto_state.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/max_expr_cnt/maxexpr.go: $(TEST_DIR)/max_expr_cnt/maxexpr.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/labeled_failures/labeled_failures.go: $(TEST_DIR)/labeled_failures/labeled_failures.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/thrownrecover/thrownrecover.go: $(TEST_DIR)/thrownrecover/thrownrecover.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/alternate_entrypoint/altentry.go: $(TEST_DIR)/alternate_entrypoint/altentry.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-grammar -alternate-entrypoints Entry2,Entry3,C $< > $@

$(TEST_DIR)/state/state.go: $(TEST_DIR)/state/state.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-grammar $< > $@

$(TEST_DIR)/stateclone/stateclone.go: $(TEST_DIR)/stateclone/stateclone.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/statereadonly/statereadonly.go: $(TEST_DIR)/statereadonly/statereadonly.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/staterestore/staterestore.go: $(TEST_DIR)/staterestore/staterestore.peg $(TEST_DIR)/staterestore/standard/staterestore.go $(TEST_DIR)/staterestore/optimized/staterestore.go $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/staterestore/standard/staterestore.go: $(TEST_DIR)/staterestore/staterestore.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/staterestore/optimized/staterestore.go: $(TEST_DIR)/staterestore/staterestore.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-grammar -optimize-parser -alternate-entrypoints TestAnd,TestNot $< > $@

$(TEST_DIR)/emptystate/emptystate.go: $(TEST_DIR)/emptystate/emptystate.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/issue_65/issue_65.go: $(TEST_DIR)/issue_65/issue_65.peg $(TEST_DIR)/issue_65/optimized/issue_65.go $(TEST_DIR)/issue_65/optimized-grammar/issue_65.go $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint $< > $@

$(TEST_DIR)/issue_65/optimized/issue_65.go: $(TEST_DIR)/issue_65/issue_65.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-parser -optimize-basic-latin $< > $@

$(TEST_DIR)/issue_65/optimized-grammar/issue_65.go: $(TEST_DIR)/issue_65/issue_65.peg $(BINDIR)/pigeon
	$(BINDIR)/pigeon -nolint -optimize-grammar $< > $@

lint:
	golint ./...
	go vet ./...

gometalinter:
	gometalinter ./...

cmp:
	@boot=$$(mktemp) && $(BINDIR)/bootstrap-pigeon $(PIGEON_GRAMMAR) > $$boot && \
	official=$$(mktemp) && $(BINDIR)/pigeon $(PIGEON_GRAMMAR) > $$official && \
	cmp $$boot $$official && \
	unlink $$boot && \
	unlink $$official

clean:
	rm -f $(BUILDER_DIR)/generated_static_code.go $(BUILDER_DIR)/generated_static_code_range_table.go
	rm -f $(BOOTSTRAPPIGEON_DIR)/bootstrap_pigeon.go $(ROOT)/pigeon.go $(TEST_GENERATED_SRC) $(EXAMPLES_DIR)/json/optimized/json.go $(EXAMPLES_DIR)/json/optimized-grammar/json.go $(TEST_DIR)/staterestore/optimized/staterestore.go $(TEST_DIR)/staterestore/standard/staterestore.go $(TEST_DIR)/issue_65/optimized/issue_65.go $(TEST_DIR)/issue_65/optimized-grammar/issue_65.go
	rm -rf $(BINDIR)

.PHONY: all clean lint gometalinter cmp

