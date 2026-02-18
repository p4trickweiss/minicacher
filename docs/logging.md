# Structured Logging Guide

The distributed cache now uses Go's `slog` package for structured logging, making it easier to understand Raft consensus behavior.

## Log Levels

Configure log levels with the `-log-level` flag:

- **debug**: Detailed operation logs (SET/GET/DELETE operations, FSM applies)
- **info**: Important cluster events (node joins, leader changes, bootstrap)
- **warn**: Warning conditions (proxy failures, no leader available)
- **error**: Error conditions (failed operations, join failures)

## Log Formats

### Text Format (default)
Human-readable format for development:

```bash
./distributed-cache -log-level=debug /tmp/node1
```

Example output:
```
time=2026-02-18T10:00:00.000Z level=INFO msg="starting distributed-cache" node_id=node1 http_addr=localhost:11000 raft_addr=localhost:12000
time=2026-02-18T10:00:00.001Z level=INFO msg="opening store" node_id=node1 raft_addr=localhost:12000 data_dir=/tmp/node1 bootstrap=true
time=2026-02-18T10:00:00.002Z level=INFO msg="raft instance created successfully" node_id=node1
time=2026-02-18T10:00:00.003Z level=INFO msg="bootstrapping new cluster" node_id=node1
time=2026-02-18T10:00:00.004Z level=INFO msg="cluster bootstrapped successfully" node_id=node1
```

### JSON Format
Machine-parseable format for production:

```bash
./distributed-cache -log-level=info -log-json /tmp/node1
```

Example output:
```json
{"time":"2026-02-18T10:00:00.000Z","level":"INFO","msg":"starting distributed-cache","node_id":"node1","http_addr":"localhost:11000","raft_addr":"localhost:12000"}
{"time":"2026-02-18T10:00:00.001Z","level":"INFO","msg":"opening store","node_id":"node1","raft_addr":"localhost:12000","data_dir":"/tmp/node1","bootstrap":true}
{"time":"2026-02-18T10:00:00.002Z","level":"INFO","msg":"raft instance created successfully","node_id":"node1"}
```

## Key Log Events for Raft Experimentation

### 1. Node Bootstrap (First Node)
```
level=INFO msg="bootstrapping new cluster" node_id=node1
level=INFO msg="cluster bootstrapped successfully" node_id=node1
```

### 2. Node Join (Subsequent Nodes)
```
level=INFO msg="received join request" node_id=node1 joining_node_id=node2 joining_node_addr=node2:12000 current_state=Leader
level=INFO msg="adding voter to cluster" node_id=node1 joining_node_id=node2 joining_node_addr=node2:12000
level=INFO msg="node successfully joined cluster" node_id=node1 joined_node_id=node2
```

### 3. Write Operations (SET)
```
level=DEBUG msg="applying set operation" node_id=node1 key=mykey value_len=7
level=DEBUG msg="applied set to state machine" node_id=node1 key=mykey value_len=7
level=INFO msg="SET successful" component=http_server key=mykey value_len=7
```

### 4. Leader Proxy (Follower Forwarding Write to Leader)
```
level=DEBUG msg="set rejected: not leader" node_id=node2 key=mykey current_state=Follower
level=DEBUG msg="proxying request to leader" component=http_server method=POST path=/store leader_addr=node1:11000
level=INFO msg="proxy successful" component=http_server method=POST path=/store leader_addr=node1:11000 status=201
```

### 5. Read Operations (GET)
```
level=DEBUG msg="GET successful" component=http_server key=mykey value_len=7
```

### 6. Delete Operations
```
level=DEBUG msg="applying delete operation" node_id=node1 key=mykey
level=DEBUG msg="applied delete to state machine" node_id=node1 key=mykey
level=INFO msg="DELETE successful" component=http_server key=mykey
```

## Usage Examples

### Development (Verbose, Human-Readable)
```bash
# See all operations including individual SET/GET/DELETE
./distributed-cache -log-level=debug /tmp/node1
```

### Production (JSON, Info Level)
```bash
# Only important events, parseable format
./distributed-cache -log-level=info -log-json /tmp/node1
```

### Debugging Raft Issues (Debug with specific node ID)
```bash
# Track what node2 is doing during cluster operations
./distributed-cache -id=node2 -log-level=debug -join=node1:11000 /tmp/node2
```

## Structured Fields

All logs include contextual fields that help correlate events:

- `component`: Which component generated the log (`store` or `http_server`)
- `node_id`: Which node generated the log (always present for easy filtering)
- `key`: Which cache key was operated on
- `current_state`: Raft state (Leader, Follower, Candidate)
- `joining_node_id`/`joining_node_addr`: Node joining cluster
- `leader_addr`: Address of current leader
- `error`: Error details when operations fail

**Every log line includes both `component` and `node_id`** for easy filtering in multi-node clusters.

## Tips for Experimenting with Raft

1. **Use debug level** to see state machine applies and understand replication
2. **Watch for state changes** in logs (Leader → Follower transitions)
3. **Grep for specific keys** to track their lifecycle:
   ```bash
   ./distributed-cache -log-level=debug /tmp/node1 2>&1 | grep 'key=mykey'
   ```
4. **Use JSON format** to pipe logs into analysis tools like `jq`:
   ```bash
   ./distributed-cache -log-json /tmp/node1 2>&1 | jq 'select(.msg | contains("join"))'
   ```
