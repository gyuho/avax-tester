#!/usr/bin/env bash
set -e

# in case IDE/gopls doesn't work

if ! [[ "$0" =~ scripts/fmt.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

goimports -w .
gofmt -s -w .

goimports -w ./avax
gofmt -s -w ./avax

goimports -w ./cmd
gofmt -s -w ./cmd

goimports -w ./runner
gofmt -s -w ./runner
