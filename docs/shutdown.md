# Graceful Shutdown

The distributed cache implements graceful shutdown to ensure data consistency and proper cleanup when stopping nodes.

## How It Works

When the application receives a termination signal (SIGTERM or SIGINT), it performs the following steps in order:

1. **HTTP Server Shutdown** (30-second timeout)
   - Stops accepting new connections
   - Waits for active HTTP requests to complete
   - Closes all idle connections

2. **Raft Shutdown**
   - Triggers a final snapshot of the state machine
   - Closes Raft transport connections
   - Stops background goroutines
   - Flushes any pending log entries

3. **Clean Exit**
   - Logs shutdown completion
   - Exits with status code 0

## Triggering Graceful Shutdown

### Interactive Mode
Press `Ctrl+C` to send SIGINT:
```bash
./distributed-cache /tmp/node1
# Press Ctrl+C
```

### Process Management
Send SIGTERM to the process:
```bash
./distributed-cache /tmp/node1 &
PID=$!

# Later...
kill -TERM $PID
```

### Docker
Stop the container (automatically sends SIGTERM):
```bash
docker stop raft-node1
```

### Docker Compose
Stop all nodes gracefully:
```bash
docker compose -f deployments/docker-compose.yml down
```

## Shutdown Logs

During graceful shutdown, you'll see structured logs showing the shutdown progress:

```
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="shutdown signal received, starting graceful shutdown"
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="shutting down HTTP server"
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="shutting down http server, waiting for active connections to finish" component=http_server node_id=node1
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="http server stopped accepting connections" component=http_server node_id=node1
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="HTTP server shutdown complete"
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="shutting down store"
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="shutting down raft instance" component=store node_id=node1
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="raft shutdown complete" component=store node_id=node1
time=2026-02-18T11:14:07.344+01:00 level=INFO msg="distributed-cache shutdown complete"
```

## Timeout Behavior

The shutdown process has a **30-second timeout**. If active HTTP requests don't complete within this time:

- The HTTP server forcefully closes remaining connections
- Raft shutdown proceeds immediately
- Process exits with error code 1

You can monitor timeout issues in the logs:
```
level=ERROR msg="HTTP server shutdown failed" error="context deadline exceeded"
```

## Why Graceful Shutdown Matters for Raft

### 1. Final Snapshot
When Raft shuts down gracefully, it creates a final snapshot of the state machine. This:
- Reduces recovery time on restart (no need to replay entire log)
- Ensures the latest committed state is persisted to disk
- Allows log truncation on next startup

### 2. Clean Cluster State
Graceful shutdown allows:
- Pending log entries to be committed
- Followers to receive final heartbeats
- Clean closure of network connections (prevents hanging sockets)

### 3. No Data Loss
Without graceful shutdown (e.g., `kill -9`):
- In-flight operations may be lost
- Uncommitted log entries are discarded
- Raft has to recover from last snapshot + log replay (slower startup)

## Best Practices for Experimentation

### Testing Leader Shutdown
```bash
# Start 3-node cluster
docker compose -f deployments/docker-compose.yml up -d

# Identify leader (look for is_leader=true in health checks)
curl http://localhost:11001/health

# Gracefully stop the leader
docker stop raft-node1

# Observe new leader election in remaining nodes
docker logs raft-node2
docker logs raft-node3
```

### Testing Follower Shutdown
```bash
# Stop a follower node gracefully
docker stop raft-node2

# Verify cluster still accepts writes on leader
curl -X POST http://localhost:11001/store \
  -H "Content-Type: application/json" \
  -d '{"key":"test","value":"data"}'

# Verify data is replicated to remaining follower
curl http://localhost:11003/store/test
```

### Testing Recovery After Graceful Shutdown
```bash
# Stop a node gracefully
docker stop raft-node3

# Restart it
docker start raft-node3

# Watch logs - should quickly catch up using snapshot
docker logs -f raft-node3
```

## Common Issues

### Timeout During Shutdown
If you see timeout errors:
- Long-running HTTP requests may be blocking shutdown
- Increase timeout in `main.go` if needed for your use case
- Or fix client code to respect shorter timeouts

### Raft Won't Shut Down
If Raft shutdown hangs:
- Check for deadlocks in FSM Apply operations
- Ensure no blocking operations in FSM Snapshot/Restore
- This is rare with the HashiCorp Raft library

### Force Kill Required
If graceful shutdown doesn't work at all:
- Check logs for errors in shutdown sequence
- Report issue with full logs
- As last resort: `kill -9 <pid>` (but expect data loss)
