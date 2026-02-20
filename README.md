# Distributed Cache

A distributed key-value store built using the Raft consensus algorithm. This project provides a highly available and fault-tolerant caching solution with strong consistency across multiple nodes.

## Features

- **Distributed Key-Value Store**: Stores key-value pairs across multiple nodes.
- **Raft Consensus Algorithm**: Ensures strong consistency and fault tolerance.
- **HTTP API**: Provides an easy-to-use HTTP interface for interacting with the store.
- **Structured Logging**: Built with Go's `slog` package for easy debugging and monitoring.
- **Graceful Shutdown**: Proper cleanup of Raft state and connections on shutdown.
- **Dockerized Deployment**: Includes a `docker-compose.yml` file for easy multi-node setup.
- **Health Checks**: Built-in health check endpoints for monitoring node status.

## Documentation

For more detailed information, please refer to the [documentation](docs).

## Installation

### Prerequisites

- [Go](https://golang.org/) (version 1.22+ recommended)
- [Docker](https://www.docker.com/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Task](https://taskfile.dev) (for task automation)

### Clone the Repository

```sh
git clone https://github.com/p4trickweiss/distributed-cache.git
cd distributed-cache
```

### Build and Run

This project uses [Taskfile](https://taskfile.dev) for task automation. Install Taskfile and run:

- **Run the application locally**:
  ```bash
  task run
  ```

- **Build the Docker image**:
  ```bash
  task build
  ```

- **Run the application with 3 nodes using Docker Compose**:
  ```bash
  task docker-compose
  ```

## Configuration

The distributed cache supports configuration via:
1. YAML configuration file (optional)
2. Environment variables (prefix: `DCACHE_`)
3. Built-in defaults

Configuration precedence: **Environment variables > Config file > Defaults**

### Running with Defaults

The application can run without any configuration file using sensible defaults:

```bash
./distributed-cache
# Uses: localhost:11000 (HTTP), localhost:12000 (Raft), ./data (storage)
```

### Configuration Files

Example configurations are provided in the `configs/` directory:
- `configs/node1.yaml` - Bootstrap node example
- `configs/node2.yaml` - Joining node example
- `configs/config.example.yaml` - Template file with all options

Use a config file:
```bash
./distributed-cache -config configs/node1.yaml
```

Example config file structure:
```yaml
node:
  id: "node1"
  data_dir: "./data"

http:
  bind_addr: "localhost:11000"

raft:
  bind_addr: "localhost:12000"

cluster:
  join_addr: ""  # Leave empty for bootstrap node

logging:
  level: "info"
  json: false
```

### Environment Variables

All configuration can be overridden with environment variables using the `DCACHE_` prefix:

| Config Path | Environment Variable | Default |
|------------|---------------------|---------|
| `node.id` | `DCACHE_NODE_ID` | (raft bind addr) |
| `node.data_dir` | `DCACHE_NODE_DATA_DIR` | `./data` |
| `http.bind_addr` | `DCACHE_HTTP_BIND_ADDR` | `localhost:11000` |
| `raft.bind_addr` | `DCACHE_RAFT_BIND_ADDR` | `localhost:12000` |
| `cluster.join_addr` | `DCACHE_CLUSTER_JOIN_ADDR` | (empty) |
| `logging.level` | `DCACHE_LOGGING_LEVEL` | `info` |
| `logging.json` | `DCACHE_LOGGING_JSON` | `false` |

Example:
```bash
DCACHE_LOGGING_LEVEL=debug DCACHE_NODE_ID=test1 ./distributed-cache
```
