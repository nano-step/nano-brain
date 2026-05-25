# Qdrant CLI Specification

## Purpose

CLI subcommands to manage Qdrant Docker container lifecycle and configuration, so users can set up Qdrant with a single command.

## ADDED Requirements

### Requirement: `qdrant up` starts Qdrant via Docker Compose

The `qdrant up` command SHALL start a Qdrant container using Docker Compose with persistent storage.

#### Scenario: First-time setup
- **WHEN** `npx nano-brain qdrant up` is run and no docker-compose file exists at ~/.nano-brain/
- **THEN** docker-compose.qdrant.yml is copied to ~/.nano-brain/
- **THEN** `docker compose -f ~/.nano-brain/docker-compose.qdrant.yml up -d` is executed
- **THEN** the command waits for Qdrant health check (GET http://localhost:6333/healthz, retry 5x at 2s intervals)
- **THEN** config.yml is updated: vector.provider set to "qdrant", vector.url set to "http://localhost:6333"

#### Scenario: Qdrant already running
- **WHEN** `npx nano-brain qdrant up` is run and Qdrant is already healthy
- **THEN** the command prints "Qdrant is already running" and exits successfully

#### Scenario: Docker not installed
- **WHEN** `npx nano-brain qdrant up` is run and `docker` command is not found
- **THEN** the command prints an error: "Docker is required. Install from https://docker.com" and exits with code 1

### Requirement: `qdrant down` stops Qdrant and falls back to sqlite-vec

The `qdrant down` command SHALL stop the Qdrant container and switch config back to sqlite-vec.

#### Scenario: Stop running container
- **WHEN** `npx nano-brain qdrant down` is run
- **THEN** `docker compose -f ~/.nano-brain/docker-compose.qdrant.yml down` is executed
- **THEN** config.yml is updated: vector.provider set to "sqlite-vec"
- **THEN** the command prints "Qdrant stopped. Switched to sqlite-vec. Data persists in Docker volume."

### Requirement: `qdrant status` shows health and vector count

The `qdrant status` command SHALL display Qdrant container status, collection info, and vector count.

#### Scenario: Qdrant running with vectors
- **WHEN** `npx nano-brain qdrant status` is run and Qdrant has 49000 vectors
- **THEN** output shows: container status (running), collection name, vector count (49000), dimensions (1024), index status

#### Scenario: Qdrant not running
- **WHEN** `npx nano-brain qdrant status` is run and Qdrant is not reachable
- **THEN** output shows: "Qdrant is not running. Start with: npx nano-brain qdrant up"
