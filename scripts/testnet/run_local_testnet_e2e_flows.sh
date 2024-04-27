#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

TELEPORTER_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd ../.. && pwd
)

set -a

source "$TELEPORTER_PATH"/scripts/versions.sh
source $TELEPORTER_PATH/.env
source $TELEPORTER_PATH/.env.local-testnet
# add /go/bin to PATH
export PATH=$PATH:/go/bin

# eval "$(head -n 89 vars.sh)"


dlv debug --headless --log --listen=:2345 --api-version=2 --accept-multiclient tests/testnet/main/run_testnet_flows.go
