import { describe, test, expect } from "bun:test";
import {
  NODES,
  sleep,
  setKey,
  getKey,
  deleteKey,
  uniqueKey,
  waitForAllNodesHealthy,
  findLeader,
} from "./helpers";

describe("Error Handling", () => {
  test("should reject SET with missing key", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value: "test" }), // Missing key
    });

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);

    const error = await res.json();
    expect(error.error).toContain("key");
  });

  test("should reject SET with missing value", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: "test" }), // Missing value
    });

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);

    const error = await res.json();
    expect(error.error).toContain("value");
  });

  test("should reject SET with empty key", async () => {
    await waitForAllNodesHealthy();

    const res = await setKey(NODES[0]!.url, "", "value");

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);
  });

  test("should reject SET with empty value", async () => {
    await waitForAllNodesHealthy();

    const res = await setKey(NODES[0]!.url, "key", "");

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);
  });

  test("should reject malformed JSON", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{ invalid json }",
    });

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);
  });

  test("should handle non-existent routes gracefully", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/nonexistent-route`);

    expect(res.ok).toBe(false);
    expect(res.status).toBe(404);
  });

  test("should return empty value for non-existent key (not error)", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("does-not-exist");
    const res = await fetch(`${NODES[0]!.url}/store/${key}`);

    expect(res.ok).toBe(true);

    const data = await res.json();
    expect(data.value).toBe("");
  });

  test("should handle DELETE of non-existent key gracefully", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("never-existed");
    const res = await deleteKey(NODES[0]!.url, key);

    // DELETE of non-existent key should succeed (idempotent)
    expect(res.ok).toBe(true);
  });
});

describe("Network Error Scenarios", () => {
  test("should timeout on unreachable node", async () => {
    // Try to connect to non-existent node
    const fakeNodeUrl = "http://localhost:99999";

    try {
      await fetch(`${fakeNodeUrl}/health`, {
        signal: AbortSignal.timeout(1000),
      });
      expect(true).toBe(false); // Should not reach here
    } catch (error) {
      expect(error).toBeDefined();
    }
  });

  test("should handle invalid node URL gracefully", async () => {
    const invalidUrl = "not-a-valid-url";

    try {
      await fetch(invalidUrl);
      expect(true).toBe(false); // Should not reach here
    } catch (error) {
      expect(error).toBeDefined();
    }
  });
});

describe("JOIN Endpoint Validation", () => {
  test("should reject JOIN with missing id", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ addr: "newnode:12000" }),
    });

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);
  });

  test("should reject JOIN with missing addr", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id: "newnode" }),
    });

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);
  });

  test("should reject JOIN with malformed JSON", async () => {
    await waitForAllNodesHealthy();

    const res = await fetch(`${NODES[0]!.url}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{ invalid }",
    });

    expect(res.ok).toBe(false);
    expect(res.status).toBe(400);
  });
});

describe("Concurrency Edge Cases", () => {
  test("should handle rapid successive updates to same key", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const key = uniqueKey("rapid-update");

    // Fire off 10 rapid updates
    const updates = [];
    for (let i = 0; i < 10; i++) {
      updates.push(setKey(leader!.url, key, `value-${i}`));
    }

    const results = await Promise.all(updates);

    // All updates should succeed
    for (const res of results) {
      expect(res.ok).toBe(true);
    }

    await sleep(1500);

    // Should converge to one of the values (last write wins in Raft)
    const data = await getKey(leader!.url, key);
    expect(data.value).toMatch(/^value-\d$/);
  });

  test("should handle concurrent writes and deletes", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("write-delete-race");

    // Concurrent write and delete
    const operations = [
      setKey(NODES[0]!.url, key, "value1"),
      setKey(NODES[1]!.url, key, "value2"),
      deleteKey(NODES[2]!.url, key),
    ];

    await Promise.all(operations);
    await sleep(1500);

    // Final state should be consistent across all nodes
    const values = await Promise.all(
      NODES.map((node) => getKey(node.url, key))
    );

    const firstValue = values[0]!.value;
    for (const data of values) {
      expect(data.value).toBe(firstValue);
    }

    // Value is either empty (delete won) or one of the writes
    expect(["", "value1", "value2"]).toContain(firstValue);
  });
});

describe("Large Operation Batches", () => {
  test("should handle many concurrent writes without errors", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const numWrites = 30;

    const writes = [];
    for (let i = 0; i < numWrites; i++) {
      writes.push(setKey(leader!.url, uniqueKey(`batch-${i}`), `val-${i}`));
    }

    const results = await Promise.all(writes);

    // Count successes
    const successCount = results.filter((r) => r.ok).length;
    const errorCount = results.filter((r) => !r.ok).length;

    console.log(
      `Batch results: ${successCount} success, ${errorCount} errors`
    );

    // Should have high success rate (allow for some network variability)
    expect(successCount).toBeGreaterThan(numWrites * 0.9);
  });
});
