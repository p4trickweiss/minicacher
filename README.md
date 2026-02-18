# Distributed Cache

A distributed key-value store built using the Raft consensus algorithm. This project provides a highly available and fault-tolerant caching solution with strong consistency across multiple nodes.

## Features

- **Distributed Key-Value Store**: Stores key-value pairs across multiple nodes.
- **Raft Consensus Algorithm**: Ensures strong consistency and fault tolerance.
- **HTTP API**: Provides an easy-to-use HTTP interface for interacting with the store.
- **Dockerized Deployment**: Includes a `docker-compose.yml` file for easy multi-node setup.
- **Health Checks**: Built-in health check endpoints for monitoring node status.

## Documentation

For more detailed information, please refer to the [documentation](docs).

## Installation

### Prerequisites

- [Go](https://golang.org/) (version 1.22+ recommended)
- [Docker](https://www.docker.com/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Protocol Buffers Compiler](https://grpc.io/docs/protoc-installation/) (for gRPC code generation)
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
