#!/usr/bin/env bash
set -e

go build godoserv.go
exec ./godoserv "$@"
