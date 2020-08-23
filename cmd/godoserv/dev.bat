@echo off
go build godoserv.go && (
    godoserv.exe %*
)
