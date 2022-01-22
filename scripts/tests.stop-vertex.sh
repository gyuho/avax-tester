#!/usr/bin/env bash
set -e

# e.g.,
# ./scripts/tests.stop-vertex.sh 1.7.3
#
# to keep the cluster alive
# SHUTDOWN=false ./scripts/tests.stop-vertex.sh 1.7.3
if ! [[ "$0" =~ scripts/tests.stop-vertex.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# TODO
