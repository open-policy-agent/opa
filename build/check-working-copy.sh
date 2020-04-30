#!/usr/bin/env bash

if output=$(git status --porcelain) && [ -z "$output" ]; then
  exit 0
else
  git status
  exit 1
fi