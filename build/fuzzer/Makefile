.PHONY: all
all: clean deps build fuzz

.PHONY: clean
clean:
	rm -f *.zip
	rm -fr workdir


.PHONY: deps
deps:
	go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build

.PHONY: build
build:
	go-fuzz-build github.com/open-policy-agent/fuzz-opa

.PHONY: fuzz
fuzz:
	go-fuzz -bin=fuzz-fuzz.zip -workdir=workdir
