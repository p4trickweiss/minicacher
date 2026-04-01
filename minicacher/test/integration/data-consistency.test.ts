import { describe, test, expect } from "bun:test";
import {
  NODES,
  sleep,
  setKey,
  getKey,
  deleteKey,
  uniqueKey,
  waitForAllNodesHealthy,
  verifyKeyOnAllNodes,
  findLeader,
} from "./helpers";

describe("Data Consistency", () => {
  test("should maintain consistency across all nodes", async () => {
    await waitForAllNodesHealthy();

    const operations = [
      { key: uniqueKey("consistency-1"), value: "value1" },
      { key: uniqueKey("consistency-2"), value: "value2" },
      { key: uniqueKey("consistency-3"), value: "value3" },
    ];

    // Perform writes
    for (const { key, value } of operations) {
      await setKey(NODES[0]!.url, key, value);
    }

    await sleep(2000); // Allow full replication

    // Verify all keys on all nodes
    for (const { key, value } of operations) {
      const allMatch = await verifyKeyOnAllNodes(key, value);
      expect(allMatch).toBe(true);
    }
  });

  test("should handle sequential updates correctly", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("sequential");
    const updates = ["v1", "v2", "v3", "v4", "v5"];

    // Sequential updates
    for (const value of updates) {
      await setKey(NODES[0]!.url, key, value);
      await sleep(300);
    }

    await sleep(1000);

    // All nodes should have final value
    const finalValue = updates[updates.length - 1]!;
    const allMatch = await verifyKeyOnAllNodes(key, finalValue);
    expect(allMatch).toBe(true);
  });

  test("should maintain consistency with mixed operations", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("mixed-ops");

    // Set
    await setKey(NODES[0]!.url, key, "initial");
    await sleep(500);

    // Update
    await setKey(NODES[1]!.url, key, "updated");
    await sleep(500);

    // Verify all nodes have updated value
    let allMatch = await verifyKeyOnAllNodes(key, "updated");
    expect(allMatch).toBe(true);

    // Delete
    await deleteKey(NODES[2]!.url, key);
    await sleep(500);

    // Verify all nodes have deleted value (empty string)
    allMatch = await verifyKeyOnAllNodes(key, "");
    expect(allMatch).toBe(true);
  });

  test("should handle burst writes correctly", async () => {
    await waitForAllNodesHealthy();

    const leader = await findLeader();
    const numWrites = 10;
    const keys: Array<{ key: string; value: string }> = [];

    // Generate key-value pairs
    for (let i = 0; i < numWrites; i++) {
      keys.push({
        key: uniqueKey(`burst-${i}`),
        value: `value-${i}`,
      });
    }

    // Perform burst writes
    const startTime = Date.now();
    await Promise.all(
      keys.map(({ key, value }) => setKey(leader!.url, key, value))
    );
    const writeTime = Date.now() - startTime;

    console.log(`Burst of ${numWrites} writes took ${writeTime}ms`);

    // Allow replication
    await sleep(2000);

    // Verify all writes replicated correctly
    for (const { key, value } of keys) {
      const allMatch = await verifyKeyOnAllNodes(key, value);
      expect(allMatch).toBe(true);
    }
  });

  test("should maintain consistency with writes from different nodes", async () => {
    await waitForAllNodesHealthy();

    const keys = [
      { node: 0, key: uniqueKey("node0-write"), value: "from-node0" },
      { node: 1, key: uniqueKey("node1-write"), value: "from-node1" },
      { node: 2, key: uniqueKey("node2-write"), value: "from-node2" },
    ];

    // Write from different nodes
    await Promise.all(
      keys.map(({ node, key, value }) => setKey(NODES[node]!.url, key, value))
    );

    await sleep(2000);

    // Verify all writes replicated to all nodes
    for (const { key, value } of keys) {
      const allMatch = await verifyKeyOnAllNodes(key, value);
      expect(allMatch).toBe(true);
    }
  });

  test("should ensure linearizability for writes", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("linearizable");
    const leader = await findLeader();

    // Write value1
    await setKey(leader!.url, key, "value1");

    // Immediately write value2 (should overwrite)
    await setKey(leader!.url, key, "value2");

    await sleep(1500);

    // All nodes must have value2 (the later write)
    // Not value1, and not sometimes value1 sometimes value2
    const values = await Promise.all(
      NODES.map((node) => getKey(node.url, key))
    );

    for (const data of values) {
      expect(data).not.toBeNull();
      expect(data!.value).toBe("value2");
    }
  });

  test("should handle overlapping key updates", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("overlap");

    // Start multiple updates to same key from different nodes
    const updates = [
      setKey(NODES[0]!.url, key, "update1"),
      setKey(NODES[1]!.url, key, "update2"),
      setKey(NODES[2]!.url, key, "update3"),
    ];

    await Promise.all(updates);
    await sleep(2000);

    // All nodes should converge to the same value
    const values = await Promise.all(
      NODES.map((node) => getKey(node.url, key))
    );

    for (const data of values) {
      expect(data).not.toBeNull();
    }

    const firstValue = values[0]!.value;
    for (const data of values) {
      expect(data!.value).toBe(firstValue);
    }

    // The value should be one of the updates
    expect(["update1", "update2", "update3"]).toContain(firstValue);
  });

  test("should preserve data integrity across delete operations", async () => {
    await waitForAllNodesHealthy();

    const keys = [
      { key: uniqueKey("preserve-1"), value: "keep1" },
      { key: uniqueKey("preserve-2"), value: "delete-me" },
      { key: uniqueKey("preserve-3"), value: "keep2" },
    ];

    // Write all keys
    for (const { key, value } of keys) {
      await setKey(NODES[0]!.url, key, value);
    }

    await sleep(1000);

    // Delete middle key
    await deleteKey(NODES[1]!.url, keys[1]!.key);

    await sleep(1000);

    // Verify deleted key is gone on all nodes
    const deletedMatch = await verifyKeyOnAllNodes(keys[1]!.key, "");
    expect(deletedMatch).toBe(true);

    // Verify other keys still exist on all nodes
    const keep1Match = await verifyKeyOnAllNodes(keys[0]!.key, "keep1");
    const keep2Match = await verifyKeyOnAllNodes(keys[2]!.key, "keep2");

    expect(keep1Match).toBe(true);
    expect(keep2Match).toBe(true);
  });
});
