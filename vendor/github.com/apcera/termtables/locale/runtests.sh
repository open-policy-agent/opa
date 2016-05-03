#!/bin/sh
# Copyright 2013 Apcera Inc. All rights reserved.

# We are checking that invoking a program gives it the correct locale
# for running, which relies upon program initialization checks, which
# mean that we can't reliably test from within one process.
#
# Instead, we test by repeated invocation.

setup() {
  rm -f ./wrapper
  go build wrapper.go
  if ! [ -x ./wrapper ]; then
    echo >&2 "Missing: ./wrapper"
    exit 1
  fi

  export LANG=''
  export LC_COLLATE=''
  export LC_CTYPE=''
  export LC_MESSAGES=''
  export LC_MONETARY=''
  export LC_NUMERIC=''
  export LC_TIME=''
  unset LC_ALL

  errcount=0
}

check_once_inner() {
  local loclabel="$1" locval="$2" expect="$3"

  have="$(./wrapper)"
  if [ "$expect" != "$have" ]; then
    echo >&2 "ERROR: with $loclabel $locval got charmap '$have', expected '$expect'"
    errcount=$((errcount + 1))
  else
    echo "Ok: $loclabel $locval -> $expect"
  fi
}

check_once_ctype() {
  local ctype="$1" expect="$2"
  export LC_CTYPE="$ctype"
  check_once_inner CTYPE "$ctype" "$expect"
  unset LC_CTYPE
}

check_once_lang() {
  local lval="$1" expect="$2"
  export LANG="$lval"
  check_once_inner LANG "$lval" "$expect"
  unset LANG
}


setup
check_once_ctype en_US.UTF-8      UTF-8
check_once_ctype C                US-ASCII
check_once_ctype en_US.ISO8859-1  ISO8859-1
check_once_lang  en_US.UTF-8      UTF-8
check_once_lang  C                US-ASCII
check_once_lang  POSIX            US-ASCII
check_once_lang  en_US.ISO8859-1  ISO8859-1

if [ $errcount -ne 0 ]; then
  echo >&2 "Saw $errcount errors"
  exit 1
fi
exit 0
