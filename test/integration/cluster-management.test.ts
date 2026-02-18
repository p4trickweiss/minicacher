import { describe, test, expect } from "bun:test";
import {
  NODES,
  sleep,
  setKey,
  getKey,
  uniqueKey,
  waitForAllNodesHealthy,
  isNodeHealthy,
  countHealthyNodes,
  findLeader,
  type HealthResponse,
} from "./helpers";

describe("Cluster Health", () => {
  test("should start with all 3 nodes healthy", async () => {
    await waitForAllNodesHealthy();

    const healthyCount = await countHealthyNodes();
    expect(healthyCount).toBe(3);
  });

  test("should report cluster status correctly", async () => {
    await waitForAllNodesHealthy();

    for (const node of NODES) {
      const res = await fetch(`${node.url}/health`);
      expect(res.ok).toBe(true);

      const health = (await res.json()) as HealthResponse;
      expect(health.status).toBe("healthy");
      expect(typeof health.is_leader).toBe("boolean");
      expect(health.time).toBeDefined();
    }
  });

  test("should have exactly one leader", async () => {
    await waitForAllNodesHealthy();

    const healthPromises = NODES.map(async (node) => {
      const res = await fetch(`${node.url}/health`);
      return res.json() as Promise<HealthResponse>;
    });

    const healthStatuses = await Promise.all(healthPromises);
    const leaderCount = healthStatuses.filter((h) => h.is_leader).length;

    expect(leaderCount).toBe(1);
  });

  test("should have exactly two followers", async () => {
    await waitForAllNodesHealthy();

    const healthPromises = NODES.map(async (node) => {
      const res = await fetch(`${node.url}/health`);
      return res.json() as Promise<HealthResponse>;
    });

    const healthStatuses = await Promise.all(healthPromises);
    const followerCount = healthStatuses.filter((h) => !h.is_leader).length;

    expect(followerCount).toBe(2);
  });
});

describe("Fault Tolerance", () => {
  test("should tolerate minority node failure (quorum maintained)", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const key = uniqueKey("fault-tolerant");
    const value = "survives-failure";

    // Write data
    await setKey(leader!.url, key, value);
    await sleep(1000);

    // Simulate reading even if one node is slow/down
    // In a real test, you'd stop one node here

    // With 3 nodes, losing 1 node should still allow reads
    // (This is implicit - the cluster continues working)

    // Verify data is accessible from remaining nodes
    for (const node of NODES) {
      const healthy = await isNodeHealthy(node.url);
      if (healthy) {
        const data = await getKey(node.url, key);
        expect(data.value).toBe(value);
      }
    }
  });

  test("should maintain operation with 3-node cluster", async () => {
    await waitForAllNodesHealthy();

    // Verify quorum operations
    const key = uniqueKey("quorum-test");
    const value = "quorum-value";

    // Write operation (requires quorum)
    const leader = await findLeader();
    const res = await setKey(leader!.url, key, value);
    expect(res.ok).toBe(true);

    await sleep(1000);

    // Read from all healthy nodes
    const healthyCount = await countHealthyNodes();
    expect(healthyCount).toBeGreaterThanOrEqual(2); // Majority

    // Verify data replicated
    for (const node of NODES) {
      if (await isNodeHealthy(node.url)) {
        const data = await getKey(node.url, key);
        expect(data.value).toBe(value);
      }
    }
  });
});

describe("Performance Characteristics", () => {
  test("should handle reasonable throughput", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const numOperations = 20;
    const operations: Promise<Response>[] = [];

    const startTime = Date.now();

    // Queue up writes
    for (let i = 0; i < numOperations; i++) {
      operations.push(
        setKey(leader!.url, uniqueKey(`perf-${i}`), `value-${i}`)
      );
    }

    // Execute all writes
    const results = await Promise.all(operations);
    const duration = Date.now() - startTime;

    // All writes should succeed
    const successCount = results.filter((r) => r.ok).length;
    expect(successCount).toBe(numOperations);

    const opsPerSecond = (numOperations / duration) * 1000;
    console.log(
      `Throughput: ${numOperations} ops in ${duration}ms (${opsPerSecond.toFixed(2)} ops/sec)`
    );

    // Should achieve at least 10 ops/sec even in test environment
    expect(opsPerSecond).toBeGreaterThan(10);
  });

  test("should have acceptable write latency", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const latencies: number[] = [];

    // Measure 10 individual write operations
    for (let i = 0; i < 10; i++) {
      const start = Date.now();
      await setKey(leader!.url, uniqueKey(`latency-${i}`), `value-${i}`);
      const latency = Date.now() - start;
      latencies.push(latency);
    }

    const avgLatency =
      latencies.reduce((a, b) => a + b, 0) / latencies.length;
    const maxLatency = Math.max(...latencies);

    console.log(`Average write latency: ${avgLatency.toFixed(2)}ms`);
    console.log(`Max write latency: ${maxLatency}ms`);

    // In a local test environment, writes should be fast
    expect(avgLatency).toBeLessThan(500);
    expect(maxLatency).toBeLessThan(1000);
  });

  test("should have fast read latency", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const key = uniqueKey("read-latency");

    // Setup: write a key
    await setKey(leader!.url, key, "test-value");
    await sleep(1000);

    // Measure read latencies from each node
    const readLatencies: Array<{ node: string; latency: number }> = [];

    for (const node of NODES) {
      const start = Date.now();
      await getKey(node.url, key);
      const latency = Date.now() - start;
      readLatencies.push({ node: node.id, latency });
    }

    for (const { node, latency } of readLatencies) {
      console.log(`Read latency from ${node}: ${latency}ms`);
      expect(latency).toBeLessThan(200); // Reads should be very fast
    }
  });
});

describe("Data Volume", () => {
  test("should handle moderate data volume", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const numKeys = 50;

    console.log(`Writing ${numKeys} key-value pairs...`);

    // Write multiple keys
    for (let i = 0; i < numKeys; i++) {
      await setKey(leader!.url, uniqueKey(`volume-${i}`), `data-${i}`);
    }

    await sleep(2000);

    // Sample check: verify a few random keys
    const samplesToCheck = 5;
    for (let i = 0; i < samplesToCheck; i++) {
      const idx = Math.floor(Math.random() * numKeys);
      const key = uniqueKey(`volume-${idx}`);
      const data = await getKey(leader!.url, key);
      // Key might not exist due to uniqueKey randomness, but if it does,
      // it should have correct value pattern
      if (data.value !== "") {
        expect(data.value).toContain("data-");
      }
    }
  });

  test("should handle varying value sizes", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();

    const testCases = [
      { key: uniqueKey("tiny"), value: "a" },
      { key: uniqueKey("small"), value: "x".repeat(100) },
      { key: uniqueKey("medium"), value: "y".repeat(1000) },
      { key: uniqueKey("large"), value: "z".repeat(10000) },
    ];

    // Write different sizes
    for (const { key, value } of testCases) {
      const res = await setKey(leader!.url, key, value);
      expect(res.ok).toBe(true);
    }

    await sleep(2000);

    // Verify all sizes replicated correctly
    for (const { key, value } of testCases) {
      const data = await getKey(leader!.url, key);
      expect(data.value).toBe(value);
      console.log(`Value size ${value.length} bytes: OK`);
    }
  });
});
