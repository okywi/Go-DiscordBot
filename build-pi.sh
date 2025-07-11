#!/bin/bash
GOOS=linux GOARCH=arm64 GOARM=7 go build -o build/GoBot cmd/bot/main.go
echo "Built for Raspberry Pi (ARMv7)"