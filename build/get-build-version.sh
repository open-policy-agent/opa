#!/usr/bin/env bash

awk -F'"' '/^var Version/{print $2}' v1/version/version.go