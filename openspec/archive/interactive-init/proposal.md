# Proposal: Interactive Init Wizard

## Why

First-time users must manually create `~/.nano-brain/config.yml` or rely on defaults that may not match their environment. An interactive setup wizard reduces friction from install to working server.

## Problem

`nano-brain init --root=<path>` only registers a workspace (requires running server). There is no guided first-time setup for config creation. Users must read docs to know what config values exist and what they mean.

## Solution

Add an interactive mode to `nano-brain init` (no flags) that prompts for essential config values, writes `config.yml`, and runs doctor checks to verify the environment.

## Scope

- New file: `cmd/nano-brain/init.go` (interactive wizard function)
- Modify: `cmd/nano-brain/commands.go` (detect interactive vs --root mode)
- No database schema changes
- No API changes
- No new dependencies (stdlib only: bufio, fmt, os, strings)

## Risk Classification

Tiny lane (1 flag: multi-file change, no external providers, no schema).
