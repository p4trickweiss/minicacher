import { describe, test, expect } from "bun:test";
import {
  NODES,
  sleep,
  setKey,
  getKey,
  findLeader,
  waitForLeader,
  findFollowers,
  uniqueKey,
  waitForAllNodesHealthy,
  verifyKeyOnAllNodes,
} from "./helpers";

describe("Leader Election and Failover", () => {
  test("should have exactly one leader after cluster starts", async () => {
    await waitForAllNodesHealthy();

    const leader = await waitForLeader();
    expect(leader).not.toBeNull();

    // Verify only one leader exists
    const healthChecks = await Promise.all(
      NODES.map(async (node) => {
        const res = await fetch(`${node.url}/health`);
        return res.json();
      })
    );

    const leaderCount = healthChecks.filter((h: any) => h.is_leader).length;
    expect(leaderCount).toBe(1);
  });

  test("should redirect writes from follower to leader", async () => {
    await waitForAllNodesHealthy();

    const followers = await findFollowers();
    expect(followers.length).toBeGreaterThan(0);

    const follower = followers[0]!;
    const key = uniqueKey("follower-write");
    const value = "redirected-value";

    // Write to follower (should be proxied to leader)
    const res = await setKey(follower.url, key, value);
    expect(res.ok).toBe(true);

    await sleep(1000);

    // Verify data was replicated
    const allMatch = await verifyKeyOnAllNodes(key, value);
    expect(allMatch).toBe(true);
  });

  test("should allow reads from any node", async () => {
    await waitForAllNodesHealthy();

    const leader = await waitForLeader();
    const key = uniqueKey("read-any");
    const value = "readable-everywhere";

    // Write to leader
    await setKey(leader!.url, key, value);
    await sleep(1000);

    // Read from all nodes
    for (const node of NODES) {
      const data = await getKey(node.url, key);
      expect(data.value).toBe(value);
    }
  });

  // Note: The following tests require the ability to stop/start containers
  // They are skipped by default but documented here for manual testing

  test.skip("should elect new leader when current leader fails", async () => {
    await waitForAllNodesHealthy();

    const initialLeader = await waitForLeader();
    console.log(`Initial leader: ${initialLeader!.id}`);

    // Store data before leader failure
    const key = uniqueKey("pre-failover");
    const value = "before-failure";
    await setKey(initialLeader!.url, key, value);
    await sleep(1000);

    // TODO: Stop the leader container
    // Example: exec(`docker stop ${initialLeader.id}`)
    console.log(`Would stop ${initialLeader!.id} here`);

    // Wait for new leader election (typically 5-10 seconds)
    await sleep(8000);

    // Verify new leader was elected
    const newLeader = await waitForLeader(15000);
    expect(newLeader).not.toBeNull();
    expect(newLeader!.id).not.toBe(initialLeader!.id);

    console.log(`New leader: ${newLeader!.id}`);

    // Verify old data is still accessible
    const data = await getKey(newLeader!.url, key);
    expect(data.value).toBe(value);

    // Write new data to new leader
    const newKey = uniqueKey("post-failover");
    const newValue = "after-failure";
    await setKey(newLeader!.url, newKey, newValue);
    await sleep(1000);

    // Verify new data replicated to remaining nodes
    const followers = await findFollowers();
    for (const follower of followers) {
      const followerData = await getKey(follower.url, newKey);
      expect(followerData.value).toBe(newValue);
    }

    // TODO: Restart the old leader
    // It should rejoin as a follower and catch up
  });

  test.skip("should handle split-brain scenarios correctly", async () => {
    // This test would require network partitioning
    // In a proper Raft implementation:
    // 1. Partition the cluster (e.g., node1 | node2,node3)
    // 2. The majority partition (node2,node3) can elect a leader
    // 3. The minority partition (node1) cannot elect a leader
    // 4. Writes to minority should fail
    // 5. When partition heals, minority catches up

    // This requires orchestration tools not available in basic test setup
    console.log("Split-brain test requires network partition tooling");
  });
});

describe("Leader Characteristics", () => {
  test("leader should accept writes immediately", async () => {
    await waitForAllNodesHealthy();

    const leader = await waitForLeader();
    const key = uniqueKey("leader-write");
    const value = "leader-value";

    const start = Date.now();
    const res = await setKey(leader!.url, key, value);
    const duration = Date.now() - start;

    expect(res.ok).toBe(true);
    console.log(`Leader write took ${duration}ms`);

    // Leader writes should be reasonably fast (< 1 second in normal conditions)
    expect(duration).toBeLessThan(1000);
  });

  test("follower writes should be proxied (may take longer)", async () => {
    await waitForAllNodesHealthy();

    const followers = await findFollowers();
    if (followers.length === 0) {
      console.log("No followers available for test");
      return;
    }

    const follower = followers[0]!;
    const key = uniqueKey("follower-proxy");
    const value = "proxied-value";

    const start = Date.now();
    const res = await setKey(follower.url, key, value);
    const duration = Date.now() - start;

    expect(res.ok).toBe(true);
    console.log(`Follower proxied write took ${duration}ms`);

    // Proxied writes may take slightly longer but should still be fast
    expect(duration).toBeLessThan(2000);
  });

  test("health endpoint should indicate leader status", async () => {
    await waitForAllNodesHealthy();

    const leader = await waitForLeader();
    const followers = await findFollowers();

    // Verify leader reports is_leader=true
    const leaderHealth = await fetch(`${leader!.url}/health`);
    const leaderData = await leaderHealth.json();
    expect(leaderData.is_leader).toBe(true);

    // Verify followers report is_leader=false
    for (const follower of followers) {
      const followerHealth = await fetch(`${follower.url}/health`);
      const followerData = await followerHealth.json();
      expect(followerData.is_leader).toBe(false);
    }
  });
});
