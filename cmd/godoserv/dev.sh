#!/usr/bin/env bash
set -e

export FIRESTORE_EMULATOR_HOST=127.0.0.1:8686

go build godoserv.go
exec ./godoserv "$@"
