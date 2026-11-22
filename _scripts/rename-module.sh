#!/bin/sh
# Renames the module from github.com/go-delve/delve to the fork's path, so
# that it can be required as a module of its own: a replace directive only
# applies in the main module, and go install refuses one. After a rebase on
# upstream, drop the old rename commit and run this again.
set -eu
cd "$(dirname "$0")/.."
from=github.com/go-delve/delve
to=${1:-github.com/ChrisRx/delve}
sed -i "s|^module $from\$|module $to|" go.mod
find . -path ./vendor -prune -o -path ./.git -prune -o -name '*.go' -print0 |
  xargs -0 grep -lZ "$from" |
  xargs -0 sed -i "s|$from|$to|g"
