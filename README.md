# TupoChat

Simple TCP chat server built with Go and PostgreSQL, featuring rooms and message history.

[![CI/CD Pipeline](https://github.com/timo0n22/tupochat/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/timo0n22/tupochat/actions)

## Features

- TCP server with SHA-256 authentication
- Room system (create, delete, join)
- Message history (500 messages per room)
- Docker and Kubernetes ready
- Automated CI/CD with GitHub Actions
- Zero-downtime deployments

# Connect
nc localhost 5522

# Commands

/help - Show available Commands
/room <name> - Create and join rooms
/join <name> - Join existing rooms
/list - List all rooms
/deleteRoom <name> - Delete your rooms
/exit - Exit chat

# Tech Stack

Go 1.24
PostgreSQL 16
Docker / Kubernetes
GitHub Actions
