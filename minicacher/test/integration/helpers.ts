/**
 * Helper utilities for integration tests
 */

export const NODES = [
  { id: "node1", url: "http://localhost:11001", raftAddr: "node1:12000" },
  { id: "node2", url: "http://localhost:11002", raftAddr: "node2:12000" },
  { id: "node3", url: "http://localhost:11003", raftAddr: "node3:12000" },
];

export const sleep = (ms: number) =>
  new Promise((resolve) => setTimeout(resolve, ms));

export interface HealthResponse {
  status: string;
  is_leader: boolean;
  time: string;
}

export interface StoreResponse {
  key: string;
  value: string;
}

export interface ErrorResponse {
  error: string;
}

/**
 * Find the current leader node
 */
export async function findLeader(): Promise<typeof NODES[number] | null> {
  for (const node of NODES) {
    try {
      const res = await fetch(`${node.url}/health`);
      const health = (await res.json()) as HealthResponse;
      if (health.is_leader) {
        return node;
      }
    } catch (e) {
      // Node might be down, continue
      continue;
    }
  }
  return null;
}

/**
 * Wait for a leader to be elected
 */
export async function waitForLeader(
  timeoutMs = 10000
): Promise<typeof NODES[number]> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const leader = await findLeader();
    if (leader) {
      return leader;
    }
    await sleep(500);
  }
  throw new Error("No leader elected within timeout");
}

/**
 * Get all follower nodes (non-leaders)
 */
export async function findFollowers(): Promise<typeof NODES> {
  const followers = [];
  for (const node of NODES) {
    try {
      const res = await fetch(`${node.url}/health`);
      const health = (await res.json()) as HealthResponse;
      if (!health.is_leader) {
        followers.push(node);
      }
    } catch (e) {
      // Node might be down, skip
      continue;
    }
  }
  return followers;
}

/**
 * Set a key-value pair on a specific node
 */
export async function setKey(
  nodeUrl: string,
  key: string,
  value: string
): Promise<Response> {
  return fetch(`${nodeUrl}/store`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key, value }),
  });
}

/**
 * Get a value for a key from a specific node
 * Returns null if key doesn't exist (404)
 */
export async function getKey(
  nodeUrl: string,
  key: string
): Promise<StoreResponse | null> {
  const res = await fetch(`${nodeUrl}/store/${key}`);
  if (res.status === 404) {
    return null;
  }
  return res.json() as Promise<StoreResponse>;
}

/**
 * Check if a key exists on a specific node
 */
export async function keyExists(
  nodeUrl: string,
  key: string
): Promise<boolean> {
  const res = await fetch(`${nodeUrl}/store/${key}`, {
    method: "HEAD",
  });
  return res.ok;
}

/**
 * Delete a key from a specific node
 */
export async function deleteKey(
  nodeUrl: string,
  key: string
): Promise<Response> {
  return fetch(`${nodeUrl}/store/${key}`, {
    method: "DELETE",
  });
}

/**
 * Verify a key has the expected value on all nodes
 * If expectedValue is empty string, verifies the key doesn't exist (404)
 */
export async function verifyKeyOnAllNodes(
  key: string,
  expectedValue: string
): Promise<boolean> {
  for (const node of NODES) {
    try {
      const data = await getKey(node.url, key);

      // If we expect empty/deleted, the key should not exist (null)
      if (expectedValue === "") {
        if (data !== null) {
          console.log(
            `Expected key to not exist on ${node.id}, but got value: "${data.value}"`
          );
          return false;
        }
      } else {
        // Key should exist with the expected value
        if (data === null) {
          console.log(
            `Key does not exist on ${node.id}, expected "${expectedValue}"`
          );
          return false;
        }
        if (data.value !== expectedValue) {
          console.log(
            `Mismatch on ${node.id}: expected "${expectedValue}", got "${data.value}"`
          );
          return false;
        }
      }
    } catch (e) {
      console.log(`Failed to get key from ${node.id}:`, e);
      return false;
    }
  }
  return true;
}

/**
 * Check if a node is healthy and responding
 */
export async function isNodeHealthy(nodeUrl: string): Promise<boolean> {
  try {
    const res = await fetch(`${nodeUrl}/health`, { signal: AbortSignal.timeout(2000) });
    return res.ok;
  } catch (e) {
    return false;
  }
}

/**
 * Wait for all nodes to be healthy AND for a leader to be elected
 */
export async function waitForAllNodesHealthy(
  timeoutMs = 15000
): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const healthChecks = await Promise.all(
      NODES.map((node) => isNodeHealthy(node.url))
    );
    if (healthChecks.every((h) => h)) {
      // All nodes are healthy, now check if leader is elected
      const leader = await findLeader();
      if (leader) {
        // Give a bit more time for cluster to stabilize
        await sleep(500);
        return;
      }
    }
    await sleep(500);
  }
  throw new Error("Not all nodes became healthy within timeout");
}

/**
 * Count how many nodes are currently healthy
 */
export async function countHealthyNodes(): Promise<number> {
  let count = 0;
  for (const node of NODES) {
    if (await isNodeHealthy(node.url)) {
      count++;
    }
  }
  return count;
}

/**
 * Generate a unique key for testing
 */
export function uniqueKey(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).substring(7)}`;
}
