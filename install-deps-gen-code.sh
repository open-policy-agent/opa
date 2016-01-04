#!/usr/bin/env sh

# install dependencies
go get -u github.com/PuerkitoBio/pigeon
go get golang.org/x/tools/cmd/goimports

# generate source code for parser
pigeon src/jsonlog/jsonlog.peg | goimports > src/jsonlog/parser.go


