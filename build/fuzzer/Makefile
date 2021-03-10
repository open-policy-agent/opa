.PHONY: all
all: clean deps build fuzz

.PHONY: clean
clean:
	rm -f *.zip
	rm -fr workdir


.PHONY: deps
deps:
	go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build

# NOTE(sr): go-fuzz-build and Go 1.15 run into each other on the compiler directive
# used in xxhash. So, let's skip instrumenting that for now.
.PHONY: build
build:
	go-fuzz-build -preserve github.com/OneOfOne/xxhash github.com/open-policy-agent/fuzz-opa

.PHONY: fuzz
fuzz:
	go-fuzz -bin=fuzz-fuzz.zip -workdir=workdir
