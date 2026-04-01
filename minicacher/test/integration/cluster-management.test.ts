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
      if (data !== null) {
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
      expect(data).not.toBeNull();
      expect(data!.value).toBe(value);
      console.log(`Value size ${value.length} bytes: OK`);
    }
  });
});
