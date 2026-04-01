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

  test("should return 404 for non-existent key", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("does-not-exist");
    const res = await fetch(`${NODES[0]!.url}/store/${key}`);

    expect(res.status).toBe(404);

    const data = await res.json();
    expect(data.error).toContain("not found");
  });

  test("should handle DELETE of non-existent key gracefully", async () => {
    await waitForAllNodesHealthy();

    const key = uniqueKey("never-existed");
    const res = await deleteKey(NODES[0]!.url, key);

    // DELETE of non-existent key should succeed (idempotent)
    expect(res.ok).toBe(true);
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
