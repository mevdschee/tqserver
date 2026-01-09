# TypeScript API Worker (Bun)

A TQServer worker written in TypeScript (running on Bun) that provides an in-memory CRUD REST API service.

## Overview

This worker demonstrates how to build TQServer workers using TypeScript, Bun, and Express. It implements a complete REST API for managing items with full CRUD operations.

## Features

- ✅ **RESTful API**: Standard HTTP methods (GET, POST, PUT, DELETE)
- ✅ **In-Memory Storage**: Simple array storage for demonstration
- ✅ **Fast Runtime**: Uses Bun for high-performance execution
- ✅ **Health Checks**: Standard `/health` endpoint for TQServer
- ✅ **TQServer Integration**: Reads configuration from environment variables

## Prerequisites

- [Bun](https://bun.sh) 1.0 or higher
- TQServer running instance

## Building & Running

TQServer automatically manages the lifecycle of this worker.

1. **Install Dependencies**: `bun install` (handled by Supervisor)
2. **Run**: `bun run index.ts` (handled by Supervisor)

## API Endpoints

### Root
- **GET /prior/api/** - Service information with HTML view

### Items CRUD

- **GET /api/items** - List all items
- **GET /api/items/:id** - Get a specific item by ID
- **POST /api/items** - Create a new item
- **PUT /api/items/:id** - Update an existing item (Not implemented in demo but standard pattern)
- **DELETE /api/items/:id** - Delete an item (Not implemented in demo but standard pattern)

### Utility

- **GET /api/health** - Health check (returns JSON status)
- **GET /api/bench** - Simple benchmark endpoint

## Configuration

Edit `config/worker.yaml` to adjust:

- **path**: URL path prefix (default: `/api`)
- **type**: `bun`
- **bun.entrypoint**: Main file (default: `index.ts`)

## Development

### Project Structure

```
workers/api/
├── package.json      # Dependencies and scripts
├── index.ts          # Main application code (Express)
├── config/
│   └── worker.yaml   # Worker configuration
└── public/           # Static assets (served by proxy or express)
```

### Hot Reload

1. Edit `index.ts`
2. Save the file
3. TQServer detects the change
4. Restarts the worker with zero-downtime

## Example Usage

### List Items
```bash
curl http://localhost:8080/api/items
```

### Create Item
```bash
curl -X POST http://localhost:8080/api/items \
  -H "Content-Type: application/json" \
  -d '{"name": "New Item", "description": "Created with Bun"}'
```
