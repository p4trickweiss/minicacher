import { describe, test, expect } from "bun:test";
import {
  NODES,
  sleep,
  setKey,
  getKey,
  deleteKey,
  verifyKeyOnAllNodes,
  uniqueKey,
  waitForAllNodesHealthy,
} from "./helpers";

describe("Basic Operations", () => {
  test("should store and retrieve a key-value pair from the same node", async () => {
    await waitForAllNodesHealthy();

    const node = NODES[0]!;
    const key = uniqueKey("basic-get");
    const value = "test-value-1";

    // Store the value
    const storeRes = await setKey(node.url, key, value);
    expect(storeRes.ok).toBe(true);

    await sleep(500); // Allow replication

    // Retrieve the value
    const data = await getKey(node.url, key);
    expect(data.value).toBe(value);
  });

  test("should replicate data across all nodes", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("replicated");
    const value = "replicated-value";

    // Store on node1
    const storeRes = await setKey(NODES[0]!.url, key, value);
    expect(storeRes.ok).toBe(true);

    await sleep(1000); // Allow time for replication

    // Verify on all nodes
    const allMatch = await verifyKeyOnAllNodes(key, value);
    expect(allMatch).toBe(true);
  });

  test("should delete a key and verify deletion across all nodes", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("delete");
    const value = "delete-value";

    // Store the value on node1
    await setKey(NODES[0]!.url, key, value);
    await sleep(1000);

    // Verify it exists on node2
    let data = await getKey(NODES[1]!.url, key);
    expect(data.value).toBe(value);

    // Delete from node3
    const deleteRes = await deleteKey(NODES[2]!.url, key);
    expect(deleteRes.ok).toBe(true);

    await sleep(1000);

    // Verify deletion on all nodes (empty string means deleted)
    const allDeleted = await verifyKeyOnAllNodes(key, "");
    expect(allDeleted).toBe(true);
  });

  test("should update an existing key", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("update");
    const value1 = "original-value";
    const value2 = "updated-value";

    // Store initial value
    await setKey(NODES[0]!.url, key, value1);
    await sleep(1000);

    // Update the value from a different node
    await setKey(NODES[1]!.url, key, value2);
    await sleep(1000);

    // Verify updated value on all nodes
    const allMatch = await verifyKeyOnAllNodes(key, value2);
    expect(allMatch).toBe(true);
  });

  test("should handle concurrent writes to the same key", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("concurrent");

    // Issue multiple concurrent writes
    const writes = Promise.all([
      setKey(NODES[0]!.url, key, "value1"),
      setKey(NODES[1]!.url, key, "value2"),
      setKey(NODES[2]!.url, key, "value3"),
    ]);

    await writes;
    await sleep(1500);

    // All nodes should have the same value (one of the writes won)
    const values = await Promise.all(
      NODES.map((node) => getKey(node.url, key))
    );

    const firstValue = values[0]!.value;
    for (const result of values) {
      expect(result.value).toBe(firstValue);
    }

    // Value should be one of the written values
    expect(["value1", "value2", "value3"]).toContain(firstValue);
  });

  test("should handle multiple keys simultaneously", async () => {
    await waitForAllNodesHealthy();

    const keys = [
      { key: uniqueKey("multi-1"), value: "value1" },
      { key: uniqueKey("multi-2"), value: "value2" },
      { key: uniqueKey("multi-3"), value: "value3" },
    ];

    // Write all keys
    await Promise.all(
      keys.map(({ key, value }) => setKey(NODES[0]!.url, key, value))
    );

    await sleep(1500);

    // Verify all keys on all nodes
    for (const { key, value } of keys) {
      const allMatch = await verifyKeyOnAllNodes(key, value);
      expect(allMatch).toBe(true);
    }
  });

  test("should handle empty values", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("empty");
    const value = "";

    // Store empty value (should be rejected)
    const storeRes = await setKey(NODES[0]!.url, key, value);
    expect(storeRes.ok).toBe(false);
    expect(storeRes.status).toBe(400);
  });

  test("should retrieve non-existent key as empty string", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("nonexistent");

    // Get non-existent key
    const data = await getKey(NODES[0]!.url, key);
    expect(data.value).toBe("");
  });
});
