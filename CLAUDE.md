ALWAYS READ FILES BEFORE ATTEMPTING TO EDIT THEM
WHEN YOU NEED TO MAKE MULTIPLE EDITS, READ THE FILE FIRST THEN MAKE THE EDIT 

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

- Build: `go build ./cmd/server`
- Run: `LISTEN_ADDR=:8080 go run ./cmd/server`
- Test all: `go test ./...`
- Lint: `golangci-lint run` (install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- Modules: `go mod tidy`

## Architecture

Minimal HTTP server in Go. Entry point `cmd/server/maelstrom.go` parses `Config` from env vars (LISTEN_ADDR) using envconfig, registers root handler, serves on configured port. `config/` package for shared config. Data flow: env → config → http.ListenAndServe. No DB, middleware, or tests yet.
