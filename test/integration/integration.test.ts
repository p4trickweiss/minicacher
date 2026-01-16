import { describe, test, expect } from "bun:test";

const NODES = [
  { id: "node1", url: "http://localhost:11001" },
  { id: "node2", url: "http://localhost:11002" },
  { id: "node3", url: "http://localhost:11003" },
];

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

describe("Distributed Cache Integration Tests", () => {
  test("should store and retrieve a key-value pair from the same node", async () => {
    const node = NODES[0]!!;
    const key = "test-key-1";
    const value = "test-value-1";

    // Store the value
    const storeRes = await fetch(`${node.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value }),
    });
    expect(storeRes.ok).toBe(true);

    await sleep(500); // Allow replication

    // Retrieve the value
    const getRes = await fetch(`${node.url}/store/${key}`);
    expect(getRes.ok).toBe(true);
    const data = (await getRes.json()) as { key: string; value: string };
    expect(data.value).toBe(value);
  });

  test("should replicate data across all nodes", async () => {
    const key = "replicated-key";
    const value = "replicated-value";

    // Store on node1
    const storeRes = await fetch(`${NODES[0]?.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value }),
    });
    expect(storeRes.ok).toBe(true);

    await sleep(1000); // Allow time for replication

    // Verify on all nodes
    for (const node of NODES) {
      const getRes = await fetch(`${node.url}/store/${key}`);
      expect(getRes.ok).toBe(true);
      const data = (await getRes.json()) as { key: string; value: string };
      expect(data.value).toBe(value);
    }
  });

  test("should delete a key and verify deletion across all nodes", async () => {
    const key = "delete-key";
    const value = "delete-value";

    // Store the value on node1
    await fetch(`${NODES[0]!!.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value }),
    });

    await sleep(1000);

    // Verify it exists on node2
    let getRes = await fetch(`${NODES[1]!!.url}/store/${key}`);
    expect(getRes.ok).toBe(true);

    // Delete from node3
    const deleteRes = await fetch(`${NODES[2]!!.url}/store/${key}`, {
      method: "DELETE",
    });
    expect(deleteRes.ok).toBe(true);

    await sleep(1000);

    // Verify deletion on all nodes
    // TODO
  });

  test("should update an existing key", async () => {
    const key = "update-key";
    const value1 = "original-value";
    const value2 = "updated-value";

    // Store initial value
    await fetch(`${NODES[0]!!.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value: value1 }),
    });

    await sleep(1000);

    // Update the value
    await fetch(`${NODES[1]!!.url}/store`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value: value2 }),
    });

    await sleep(1000);

    // Verify updated value on all nodes
    for (const node of NODES) {
      const getRes = await fetch(`${node.url}/store/${key}`);
      const data = (await getRes.json()) as { key: string; value: string };
      expect(data.value).toBe(value2);
    }
  });
});
