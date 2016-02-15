#!/usr/bin/env sh

# install dependencies
go get -u github.com/PuerkitoBio/pigeon
go get golang.org/x/tools/cmd/goimports

# generate source code for parser.  Delete first so no silent errors.
rm src/jsonlog/parser.go
pigeon src/jsonlog/jsonlog.peg | goimports > src/jsonlog/parser.go


