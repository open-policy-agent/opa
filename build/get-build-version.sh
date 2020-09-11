#!/usr/bin/env bash

grep '^var Version' version/version.go | awk '{print $4}' | tr -d '"'