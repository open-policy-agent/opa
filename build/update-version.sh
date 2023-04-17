#!/usr/bin/env bash

set -e
# NOTE(sr): This was the only way I've found to replace the string
# reliably on OSX and Linux.
perl -pi -e "s/Version = \".*\"$/Version = \"$1\"/" version/version.go