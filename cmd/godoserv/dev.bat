@echo off
set FIRESTORE_EMULATOR_HOST=127.0.0.1:8686

go build godoserv.go && (
    godoserv.exe %*
)
