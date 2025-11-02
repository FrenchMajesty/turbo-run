/**
 * Scenario loader - parses JSON and populates stores
 */

import { Scenario, ScenarioNode, validateScenario } from "../types/scenario";
import { getEventEmitter, ChoreographyEventType } from "./events";
import { getBudgetTracker } from "./budgetTracker";
import { useNodesStore } from "../stores/useNodesStore";
import { useQueueStore } from "../stores/useQueueStore";
import { useWorkersStore } from "../stores/useWorkersStore";
import { useStatsStore } from "../stores/useStatsStore";
import { useEventsStore } from "../stores/useEventsStore";
import { useChoreographyStore } from "../stores/choreographyStore";
import { NodeData } from "../types/NodeData";

interface GraphNode {
  node: ScenarioNode;
  indegree: number;
  dependents: string[]; // Nodes that depend on this node
}

export class ScenarioLoader {
  private emitter = getEventEmitter();
  private budgetTracker = getBudgetTracker();
  private graph: Map<string, GraphNode> = new Map();

  /**
   * Load a scenario from JSON
   */
  async loadScenario(scenarioJson: string, fileName: string): Promise<void> {
    // Parse and validate
    const data = JSON.parse(scenarioJson);
    const scenario = validateScenario(data);

    // Reset all stores
    this.reset();

    // Initialize budget tracker
    this.budgetTracker.initialize("groq", scenario.rate_limits.groq);
    this.budgetTracker.initialize("openai", scenario.rate_limits.openai);

    // Build dependency graph
    this.buildGraph(scenario);

    // Populate stores
    this.populateStores(scenario);

    // Update choreography store
    useChoreographyStore.getState().setScenario(fileName, scenario.nodes.length);

    // Emit ready events for nodes with indegree 0
    this.emitReadyNodes();
  }

  /**
   * Build dependency graph and calculate indegrees
   */
  private buildGraph(scenario: Scenario): void {
    // Initialize graph nodes
    for (const node of scenario.nodes) {
      this.graph.set(node.id, {
        node,
        indegree: node.dependencies.length,
        dependents: [],
      });
    }

    // Build dependents list (reverse edges)
    for (const node of scenario.nodes) {
      for (const depId of node.dependencies) {
        const depNode = this.graph.get(depId);
        if (depNode) {
          depNode.dependents.push(node.id);
        }
      }
    }
  }

  /**
   * Populate all stores with scenario data
   */
  private populateStores(scenario: Scenario): void {
    const nodesStore = useNodesStore.getState();

    // Add all nodes to nodes store
    for (const scenarioNode of scenario.nodes) {
      const nodeData: NodeData = {
        id: scenarioNode.id,
        status: "node_created",
        dependencies: scenarioNode.dependencies,
        provider: scenarioNode.provider,
        tokens: scenarioNode.estimated_tokens,
        type: "workNode",
        position: { x: 0, y: 0 }, // Will be laid out by graph layout
        data: {
          label: scenarioNode.id,
          provider: scenarioNode.provider,
          estimated_tokens: scenarioNode.estimated_tokens,
          dependencies: scenarioNode.dependencies,
          attempt: 0,
          max_retries: scenarioNode.duration_ms.length - 1,
        },
      };

      nodesStore.addNode(scenarioNode.id, nodeData);
    }

    // Stats will be updated by the choreographer as nodes progress
  }

  /**
   * Emit NODE_READY events for all nodes with indegree 0
   */
  private emitReadyNodes(): void {
    this.graph.forEach((graphNode, nodeId) => {
      if (graphNode.indegree === 0) {
        this.emitter.emit(ChoreographyEventType.NODE_READY, { nodeId });
      }
    });
  }

  /**
   * Decrement indegree when a dependency completes
   * Returns true if node becomes ready (indegree reaches 0)
   */
  decrementIndegree(nodeId: string): boolean {
    const graphNode = this.graph.get(nodeId);
    if (!graphNode) {
      return false;
    }

    graphNode.indegree--;
    return graphNode.indegree === 0;
  }

  /**
   * Get dependents of a node
   */
  getDependents(nodeId: string): string[] {
    return this.graph.get(nodeId)?.dependents ?? [];
  }

  /**
   * Get scenario node data
   */
  getScenarioNode(nodeId: string): ScenarioNode | null {
    return this.graph.get(nodeId)?.node ?? null;
  }

  /**
   * Check if all nodes are completed or failed
   */
  isComplete(): boolean {
    const choreography = useChoreographyStore.getState();
    const totalFinished = choreography.completedNodes + choreography.failedNodes;
    return totalFinished === choreography.totalNodes;
  }

  /**
   * Reset all state
   */
  private reset(): void {
    // Clear graph
    this.graph.clear();

    // Reset stores
    useNodesStore.getState().clearNodes();
    useQueueStore.getState().reset();
    useWorkersStore.getState().reset();
    useStatsStore.getState().reset();
    useEventsStore.getState().reset();

    // Budget tracker will be reinitialized with new configs
    this.budgetTracker.cleanup();
  }
}

// Global singleton
let globalLoader: ScenarioLoader | null = null;

/**
 * Get the global scenario loader instance
 */
export function getScenarioLoader(): ScenarioLoader {
  if (!globalLoader) {
    globalLoader = new ScenarioLoader();
  }
  return globalLoader;
}

/**
 * Reset the global scenario loader
 */
export function resetScenarioLoader(): void {
  globalLoader = null;
}
