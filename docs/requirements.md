# Distributed Cache with Raft Consensus

## Group Members
- Patrick Weiss

## 1. Project Overview
The Distributed Cache with Raft Consensus is a high-performance, fault-tolerant caching system designed to provide consistent and reliable data storage across multiple nodes. Built using Go and HashiCorp's Raft implementation, the system ensures strong consistency guarantees while maintaining high availability in distributed environments.

The cache operates as a cluster of nodes that coordinate through the Raft consensus protocol, ensuring that all nodes agree on the state of cached data even in the presence of failures. This makes it suitable for applications requiring both performance benefits of caching and strong consistency guarantees typically associated with databases.

### 1.1 Vision
To create a production-ready distributed caching solution that bridges the gap between simple in-memory caches and complex distributed databases, providing developers with a reliable, easy-to-deploy caching layer that maintains data consistency across distributed systems without sacrificing performance or availability.

### 1.2 Project Goals
- Educational Excellence: Demonstrate deep understanding of distributed systems concepts, consensus algorithms, and production Go development practices through practical implementation.
- Technical Robustness: Build a system that correctly implements the Raft consensus protocol, handles network partitions gracefully, and maintains data consistency under various failure scenarios.
- Operational Simplicity: Provide intuitive APIs, comprehensive monitoring, and simple deployment procedures that make the system accessible to developers without deep distributed systems expertise.

## 2. Functional Requirements

### 2.1 Core Cache Operations

**FR-1.1** The system SHALL provide basic cache operations:
- SET(key, value, ttl): Store a key-value pair with optional time-to-live
- GET(key): Retrieve value for a given key
- DELETE(key): Remove a key-value pair
- EXISTS(key): Check if a key exists
- EXPIRE(key, ttl): Update expiration time for existing key

**FR-1.2** The system SHALL support batch operations:
- MGET(keys[]): Retrieve multiple values in single request
- MSET(entries[]): Set multiple key-value pairs atomically
- MDEL(keys[]): Delete multiple keys in single operation

**FR-1.3** The system SHALL implement cache eviction policies:
- LRU (Least Recently Used)
- TTL-based expiration
- Size-based eviction when memory threshold is reached

### 2.2 Distributed Systems Features

**FR-2.1** The system SHALL implement Raft consensus protocol for:
- Leader election
- Log replication
- Commitment of entries
- Snapshot creation and restoration

**FR-2.2** The system SHALL support cluster operations:
- Add new nodes to existing cluster
- Remove nodes from cluster
- Replace failed nodes
- Rebalance data after topology changes

### 2.3 Client Interface

**FR-3.1** The system SHALL provide multiple client protocols:
- gRPC for high-performance binary communication
- HTTP/REST API for compatibility and debugging

**FR-3.2** The system SHALL implement request routing:
- Automatic forwarding of write requests to leader
- Client-side leader discovery
- Transparent failover during leader changes

### 2.4 Data Management

**FR-4.1** The system SHALL support data types:
- Binary data (byte arrays)
- Strings with UTF-8 encoding
- Structured data (JSON/MessagePack serialization)

**FR-4.2** The system SHALL implement data persistence:
- Periodic snapshots to disk
- Write-ahead logging for durability
- Configurable fsync policies

### 2.5 Monitoring and Administration

**FR-5.1** The system SHALL expose operational metrics:
- Cache hit/miss rates
- Operation latencies (p50, p95, p99)
- Cluster health status
- Node-specific statistics

## 3. Non-Functional Requirements

### 3.1 Performance Requirements

**NFR-1.1** **Throughput**: The system SHALL handle minimum 10,000 operations/second on a 3-node cluster with 1KB values.

**NFR-1.2** **Latency**: The system SHALL maintain:
- p50 latency < 5ms for read operations
- p50 latency < 10ms for write operations
- p99 latency < 50ms under normal load

**NFR-1.3** **Scalability**: The system SHALL scale linearly up to 7 nodes for read operations.

### 3.2 Reliability Requirements

**NFR-2.1** **Availability**: The system SHALL maintain 99.9% availability with a 3-node cluster, tolerating single node failures.

**NFR-2.2** **Fault Tolerance**: The system SHALL continue operating with (n-1)/2 node failures in an n-node cluster.

**NFR-2.3** **Recovery Time**: The system SHALL elect a new leader within 5 seconds of leader failure detection.

**NFR-2.4** **Data Durability**: The system SHALL not lose committed data during planned shutdowns or single node failures.

### 3.3 Consistency Requirements

**NFR-3.1** **Strong Consistency**: The system SHALL provide linearizable consistency for all write operations.

**NFR-3.2** **Read Consistency**: The system SHALL offer configurable read consistency with clear staleness bounds.

**NFR-3.3** **Split-brain Prevention**: The system SHALL prevent split-brain scenarios during network partitions.

### 3.4 Operational Requirements

**NFR-4.1** **Deployment**: The system SHALL be deployable via:
- Docker containers

**NFR-4.2** **Configuration**: The system SHALL support:
- YAML/TOML configuration files
- Environment variable overrides
- Runtime configuration updates for non-critical parameters

**NFR-4.3** **Observability**: The system SHALL integrate with:
- Prometheus for metrics
- OpenTelemetry for distributed tracing
- Structured logging with configurable levels

### 3.5 Security Requirements

**NFR-5.1** **Authentication**: The system SHALL support mutual TLS for node-to-node communication.

**NFR-5.2** **Encryption**: The system SHALL support encryption at rest for snapshot files.

### 3.6 Usability Requirements

**NFR-6.1** **Documentation**: The system SHALL provide:
- API reference documentation
- Deployment guides
- Architecture documentation

**NFR-6.2** **Error Handling**: The system SHALL provide clear, actionable error messages with error codes.

**NFR-6.3** **Backward Compatibility**: The system SHALL maintain API compatibility within major versions.

## 4. Requirements Prioritization (MoSCoW Method)

### 4.1 MUST Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| **FR-1.1** | Basic cache operations (SET/GET/DELETE/EXISTS/EXPIRE) | Core functionality - without these operations, the system cannot function as a cache |
| **FR-2.1** | Raft consensus implementation (leader election, log replication, commitment) | Essential for distributed consistency and fault tolerance - core project requirement |
| **FR-3.1** | At least one client protocol (gRPC) | Required for client-server communication - system unusable without it |
| **FR-4.2** | Data persistence (snapshots, WAL) | Critical for data durability and recovery after failures |
| **NFR-2.2** | Basic fault tolerance ((n-1)/2 failures) | Core value proposition of using Raft consensus |
| **NFR-3.1** | Strong consistency for writes | Fundamental correctness requirement for distributed cache |
| **NFR-6.1** | Basic documentation (API, deployment) | Necessary for project evaluation and usage |

### 4.2 SHOULD Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| **FR-1.2** | Batch operations (MGET/MSET/MDEL) | Significant performance improvement for real-world usage |
| **FR-1.3** | Cache eviction policies (LRU, TTL, size-based) | Essential for memory management in production |
| **FR-2.2** | Dynamic cluster membership | Operational necessity for maintaining clusters |
| **FR-3.2** | Request routing (leader forwarding, failover) | Improves client experience and system resilience |
| **FR-5.1** | Operational metrics | Critical for debugging and monitoring system health |
| **NFR-1.1** | Throughput target (10K ops/sec) | Demonstrates production readiness |
| **NFR-1.2** | Latency targets (p50, p99) | Performance validation |
| **NFR-2.3** | Recovery time < 5 seconds | Ensures reasonable availability during failures |
| **NFR-4.3** | Prometheus integration | Industry-standard monitoring |

### 4.3 COULD Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| **FR-3.1** | HTTP/REST API (in addition to gRPC) | Enhances compatibility and debugging capabilities |
| **FR-4.1** | Multiple data type support | Increases system versatility |
| **NFR-1.3** | Linear scalability to 7 nodes | Advanced performance characteristic |
| **NFR-2.1** | 99.9% availability guarantee | High availability target |
| **NFR-2.4** | Data durability guarantees | Enhanced reliability |
| **NFR-3.2** | Configurable read consistency | Advanced feature for performance optimization |
| **NFR-4.1** | Docker deployment | Simplifies deployment and testing |
| **NFR-4.2** | YAML/TOML configuration | Better configuration management |
| **NFR-5.1** | Mutual TLS | Security enhancement |
| **NFR-5.2** | Encryption at rest | Data security |
| **NFR-6.2** | Detailed error handling | Improved developer experience |
| **NFR-6.3** | Backward compatibility | Long-term maintainability |

### 4.4 WON'T Have

| Requirement | Description | Justification |
|------------|-------------|---------------|
| - | Web UI for monitoring/administration | Frontend development out of scope - focus on backend distributed systems |
| - | Geographic/multi-region replication | Too complex for project timeline |
| - | SQL-like query language | Beyond scope of a cache system |
| - | Multiple Raft groups (sharding) | Significant complexity increase |
| - | CRDT support | Different consistency model than Raft |
| - | Built-in data analytics | Not core to caching functionality |
| - | Kubernetes operator | Complex orchestration beyond scope |
| - | Multi-tenancy with isolation | Enterprise feature beyond scope |
| - | Advanced authentication (OAuth, SAML) | Complex security implementation |
| - | Compression algorithms | Optimization beyond core requirements |
| - | Cache warming/preloading | Advanced feature |

### 4.5 Prioritization Rationale

**MUST Have Rationale:**
- Focuses on core distributed cache functionality with Raft consensus
- Ensures system is functional and demonstrates key concepts
- Achievable within first semester timeline
- Forms foundation for all other features

**SHOULD Have Rationale:**
- Enhances system from prototype to production-viable
- Addresses real-world operational needs
- Demonstrates deeper understanding of distributed systems
- Achievable with focused effort in early second semester

**COULD Have Rationale:**
- Represents stretch goals that enhance usability and operations
- Provides differentiation if time permits
- Can be partially implemented based on progress
- Not critical for demonstrating core competencies

**WON'T Have Rationale:**
- Features that would require significant architectural changes
- Capabilities beyond typical cache systems
- Enterprise-level features requiring extensive additional work
- Features that would distract from core learning objectives
