/**
 * Type definitions for scenario JSON files
 * Scenarios define the choreography for frontend-only animations
 */

export type Provider = "groq" | "openai";
export type NodeStatus = "success" | "failed";

export interface ScenarioNode {
  /** Unique identifier for the node */
  id: string;

  /** LLM provider (groq or openai) */
  provider: Provider;

  /** Estimated token consumption for rate limiting */
  estimated_tokens: number;

  /**
   * Duration in milliseconds for each attempt
   * Array length determines number of attempts
   * E.g., [2000, 3000] = first attempt 2s, retry 3s
   */
  duration_ms: number[];

  /** Final status after all attempts */
  final_status: NodeStatus;

  /** IDs of nodes that must complete before this node can start */
  dependencies: string[];
}

export interface RateLimitConfig {
  /** Maximum tokens allowed per window */
  tokens_per_window: number;

  /** Maximum requests allowed per window */
  requests_per_window: number;

  /** Window duration in milliseconds */
  window_ms: number;
}

export interface Scenario {
  /** All nodes in the scenario */
  nodes: ScenarioNode[];

  /** Rate limit configuration per provider */
  rate_limits: {
    groq: RateLimitConfig;
    openai: RateLimitConfig;
  };
}

/**
 * Validates a scenario JSON object
 * @throws Error if validation fails
 */
export function validateScenario(data: unknown): Scenario {
  if (!data || typeof data !== "object") {
    throw new Error("Scenario must be an object");
  }

  const scenario = data as Partial<Scenario>;

  // Validate nodes array
  if (!Array.isArray(scenario.nodes)) {
    throw new Error("Scenario must have a 'nodes' array");
  }

  // Validate each node
  const nodeIds = new Set<string>();
  for (const node of scenario.nodes) {
    if (!node.id || typeof node.id !== "string") {
      throw new Error("Each node must have a string 'id'");
    }
    if (nodeIds.has(node.id)) {
      throw new Error(`Duplicate node id: ${node.id}`);
    }
    nodeIds.add(node.id);

    if (node.provider !== "groq" && node.provider !== "openai") {
      throw new Error(`Node ${node.id}: provider must be 'groq' or 'openai'`);
    }

    if (typeof node.estimated_tokens !== "number" || node.estimated_tokens < 0) {
      throw new Error(`Node ${node.id}: estimated_tokens must be a non-negative number`);
    }

    if (!Array.isArray(node.duration_ms) || node.duration_ms.length === 0) {
      throw new Error(`Node ${node.id}: duration_ms must be a non-empty array`);
    }
    for (const duration of node.duration_ms) {
      if (typeof duration !== "number" || duration < 0) {
        throw new Error(`Node ${node.id}: all duration_ms values must be non-negative numbers`);
      }
    }

    if (node.final_status !== "success" && node.final_status !== "failed") {
      throw new Error(`Node ${node.id}: final_status must be 'success' or 'failed'`);
    }

    if (!Array.isArray(node.dependencies)) {
      throw new Error(`Node ${node.id}: dependencies must be an array`);
    }
  }

  // Validate dependency references
  for (const node of scenario.nodes) {
    for (const depId of node.dependencies) {
      if (!nodeIds.has(depId)) {
        throw new Error(`Node ${node.id}: dependency '${depId}' does not exist`);
      }
    }
  }

  // Validate rate limits
  if (!scenario.rate_limits || typeof scenario.rate_limits !== "object") {
    throw new Error("Scenario must have a 'rate_limits' object");
  }

  const validateRateLimit = (provider: string, config: unknown) => {
    if (!config || typeof config !== "object") {
      throw new Error(`Rate limit for ${provider} must be an object`);
    }
    const limit = config as Partial<RateLimitConfig>;

    if (typeof limit.tokens_per_window !== "number" || limit.tokens_per_window <= 0) {
      throw new Error(`${provider}: tokens_per_window must be a positive number`);
    }
    if (typeof limit.requests_per_window !== "number" || limit.requests_per_window <= 0) {
      throw new Error(`${provider}: requests_per_window must be a positive number`);
    }
    if (typeof limit.window_ms !== "number" || limit.window_ms <= 0) {
      throw new Error(`${provider}: window_ms must be a positive number`);
    }
  };

  validateRateLimit("groq", scenario.rate_limits.groq);
  validateRateLimit("openai", scenario.rate_limits.openai);

  // Check for circular dependencies (simple DFS)
  const detectCycle = (nodeId: string, visited: Set<string>, recStack: Set<string>): boolean => {
    visited.add(nodeId);
    recStack.add(nodeId);

    const node = scenario.nodes!.find((n) => n.id === nodeId)!;
    for (const depId of node.dependencies) {
      if (!visited.has(depId)) {
        if (detectCycle(depId, visited, recStack)) {
          return true;
        }
      } else if (recStack.has(depId)) {
        return true;
      }
    }

    recStack.delete(nodeId);
    return false;
  };

  const visited = new Set<string>();
  for (const node of scenario.nodes) {
    if (!visited.has(node.id)) {
      if (detectCycle(node.id, visited, new Set())) {
        throw new Error(`Circular dependency detected involving node ${node.id}`);
      }
    }
  }

  return scenario as Scenario;
}
