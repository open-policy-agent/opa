#!/usr/bin/env bash

awk -F'"' '/^var Version/{print $2}' version/version.go