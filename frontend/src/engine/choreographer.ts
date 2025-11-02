/**
 * Choreographer - Event-driven orchestration engine
 * Listens to events and coordinates animations between stores
 */

import {
  getEventEmitter,
  ChoreographyEventType,
  ChoreographyEventPayloads,
} from "./events";
import { getScenarioLoader } from "./scenarioLoader";
import { getBudgetTracker } from "./budgetTracker";
import { useNodesStore } from "../stores/useNodesStore";
import { useQueueStore } from "../stores/useQueueStore";
import { useWorkersStore } from "../stores/useWorkersStore";
import { useStatsStore } from "../stores/useStatsStore";
import { useChoreographyStore, PlaybackState } from "../stores/choreographyStore";
import { ANIMATION_DURATION_MS } from "../constants/timing";

export class Choreographer {
  private emitter = getEventEmitter();
  private loader = getScenarioLoader();
  private budgetTracker = getBudgetTracker();
  private unsubscribers: (() => void)[] = [];
  private isPaused = false;
  private pendingAnimations: Set<Promise<void>> = new Set();

  /**
   * Initialize the choreographer and register all event listeners
   */
  initialize(): void {
    this.registerListeners();
  }

  /**
   * Register all event listeners
   */
  private registerListeners(): void {
    // Node lifecycle
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_READY, (payload) =>
        this.onNodeReady(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_ENQUEUED, (payload) =>
        this.onNodeEnqueued(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.PQ_REORDERED, (payload) =>
        this.onPQReordered(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_LAUNCHED, (payload) =>
        this.onNodeLaunched(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_DISPATCHED, (payload) =>
        this.onNodeDispatched(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_ATTEMPT_START, (payload) =>
        this.onNodeAttemptStart(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_ATTEMPT_COMPLETE, (payload) =>
        this.onNodeAttemptComplete(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_COMPLETED, (payload) =>
        this.onNodeCompleted(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.NODE_FAILED, (payload) =>
        this.onNodeFailed(payload)
      )
    );

    // Budget management
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.BUDGET_INSUFFICIENT, (payload) =>
        this.onBudgetInsufficient(payload)
      )
    );
    this.unsubscribers.push(
      this.emitter.on(ChoreographyEventType.BUDGET_RESET, (payload) =>
        this.onBudgetReset(payload)
      )
    );
  }

  /**
   * NODE_READY: Animate node into priority queue
   */
  private async onNodeReady(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_READY]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId } = payload;
    const nodesStore = useNodesStore.getState();
    const queueStore = useQueueStore.getState();

    // Update node status
    nodesStore.updateNode(nodeId, { status: "node_ready" });

    // Animate to priority queue
    const animation = this.animate(async () => {
      queueStore.addToQueue(nodeId);
      await this.emitter.emit(ChoreographyEventType.NODE_ENQUEUED, { nodeId });
    });

    this.trackAnimation(animation);
  }

  /**
   * NODE_ENQUEUED: Trigger priority queue reorder
   */
  private async onNodeEnqueued(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_ENQUEUED]): Promise<void> {
    if (this.isPaused) return;

    const queueStore = useQueueStore.getState();
    const nodesStore = useNodesStore.getState();

    // Get current queue and sort by estimated tokens (descending)
    const currentQueue = [...queueStore.priorityQueueNodes];
    const sortedQueue = currentQueue.sort((a, b) => {
      const nodeA = nodesStore.getNode(a);
      const nodeB = nodesStore.getNode(b);
      const tokensA = nodeA?.data?.estimated_tokens ?? 0;
      const tokensB = nodeB?.data?.estimated_tokens ?? 0;
      return tokensB - tokensA; // Higher tokens first
    });

    // Animate reorder
    const animation = this.animate(async () => {
      queueStore.setQueue(sortedQueue);
      await this.emitter.emit(ChoreographyEventType.PQ_REORDERED, { nodeIds: sortedQueue });
    });

    this.trackAnimation(animation);
  }

  /**
   * PQ_REORDERED: Pop highest priority node and check budget
   */
  private async onPQReordered(payload: ChoreographyEventPayloads[ChoreographyEventType.PQ_REORDERED]): Promise<void> {
    if (this.isPaused) return;

    const { nodeIds } = payload;
    if (nodeIds.length === 0) return;

    // Pop from front of queue (highest priority)
    const nodeId = nodeIds[0];
    const scenarioNode = this.loader.getScenarioNode(nodeId);
    if (!scenarioNode) return;

    await this.emitter.emit(ChoreographyEventType.NODE_LAUNCHED, {
      nodeId,
      provider: scenarioNode.provider,
    });
  }

  /**
   * NODE_LAUNCHED: Check budget and dispatch or block
   */
  private async onNodeLaunched(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_LAUNCHED]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId, provider } = payload;
    const scenarioNode = this.loader.getScenarioNode(nodeId);
    if (!scenarioNode) return;

    // Check budget
    const canLaunch = this.budgetTracker.canLaunch(provider, scenarioNode.estimated_tokens);

    if (!canLaunch) {
      // Insufficient budget - wait for reset
      await this.emitter.emit(ChoreographyEventType.BUDGET_INSUFFICIENT, { nodeId, provider });
      return;
    }

    // Record consumption
    this.budgetTracker.recordConsumption(provider, scenarioNode.estimated_tokens);

    // Update stats
    const stats = this.budgetTracker.getStats(provider);
    if (stats) {
      const statsStore = useStatsStore.getState();
      statsStore.updateStats({
        [provider]: {
          tokens_used: stats.tokensConsumed,
          requests_used: stats.requestsConsumed,
        },
      });
    }

    // Assign to random worker
    const workerId = Math.floor(Math.random() * 120);

    await this.emitter.emit(ChoreographyEventType.NODE_DISPATCHED, { nodeId, workerId });
  }

  /**
   * NODE_DISPATCHED: Animate to worker grid and start attempt
   */
  private async onNodeDispatched(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_DISPATCHED]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId, workerId } = payload;
    const queueStore = useQueueStore.getState();
    const workersStore = useWorkersStore.getState();
    const nodesStore = useNodesStore.getState();

    // Animate removal from queue and assignment to worker
    const animation = this.animate(async () => {
      queueStore.removeFromQueue(nodeId);
      workersStore.assignWorker(workerId, nodeId);
      nodesStore.updateNode(nodeId, { status: "node_running" });

      // Start first attempt
      const scenarioNode = this.loader.getScenarioNode(nodeId);
      if (scenarioNode) {
        await this.emitter.emit(ChoreographyEventType.NODE_ATTEMPT_START, {
          nodeId,
          attemptIndex: 0,
          durationMs: scenarioNode.duration_ms[0],
        });
      }
    });

    this.trackAnimation(animation);
  }

  /**
   * NODE_ATTEMPT_START: Execute work for specified duration
   */
  private async onNodeAttemptStart(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_ATTEMPT_START]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId, attemptIndex, durationMs } = payload;
    const nodesStore = useNodesStore.getState();
    const scenarioNode = this.loader.getScenarioNode(nodeId);
    if (!scenarioNode) return;

    // Update attempt count
    nodesStore.updateNode(nodeId, {
      data: {
        attempt: attemptIndex + 1,
        maxAttempts: scenarioNode.duration_ms.length,
      },
    });

    // Wait for duration (respecting playback speed)
    const speed = useChoreographyStore.getState().speed;
    const adjustedDuration = durationMs / speed;

    await this.delay(adjustedDuration);

    if (this.isPaused) return;

    // Determine success based on final_status and attempt number
    const isLastAttempt = attemptIndex === scenarioNode.duration_ms.length - 1;
    const success = isLastAttempt ? scenarioNode.final_status === "success" : false;

    await this.emitter.emit(ChoreographyEventType.NODE_ATTEMPT_COMPLETE, {
      nodeId,
      attemptIndex,
      success,
    });
  }

  /**
   * NODE_ATTEMPT_COMPLETE: Retry or complete/fail
   */
  private async onNodeAttemptComplete(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_ATTEMPT_COMPLETE]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId, attemptIndex, success } = payload;
    const scenarioNode = this.loader.getScenarioNode(nodeId);
    if (!scenarioNode) return;

    if (success) {
      // Node completed successfully
      await this.emitter.emit(ChoreographyEventType.NODE_COMPLETED, { nodeId });
    } else {
      const nextAttemptIndex = attemptIndex + 1;
      if (nextAttemptIndex < scenarioNode.duration_ms.length) {
        // Retry
        await this.emitter.emit(ChoreographyEventType.NODE_ATTEMPT_START, {
          nodeId,
          attemptIndex: nextAttemptIndex,
          durationMs: scenarioNode.duration_ms[nextAttemptIndex],
        });
      } else {
        // All retries exhausted
        await this.emitter.emit(ChoreographyEventType.NODE_FAILED, { nodeId });
      }
    }
  }

  /**
   * NODE_COMPLETED: Release worker, update dependents
   */
  private async onNodeCompleted(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_COMPLETED]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId } = payload;
    const nodesStore = useNodesStore.getState();
    const workersStore = useWorkersStore.getState();

    // Update status
    nodesStore.updateNode(nodeId, { status: "node_completed" });

    // Release worker
    workersStore.releaseWorker(nodeId);

    // Update progress
    useChoreographyStore.getState().incrementCompleted();

    // Remove from graph (not from UI, but conceptually)
    // Decrement dependents' indegree
    const dependents = this.loader.getDependents(nodeId);
    for (const dependentId of dependents) {
      const isReady = this.loader.decrementIndegree(dependentId);
      if (isReady) {
        await this.emitter.emit(ChoreographyEventType.NODE_READY, { nodeId: dependentId });
      }
    }

    // Check if all nodes complete
    if (this.loader.isComplete()) {
      useChoreographyStore.getState().setState(PlaybackState.COMPLETED);
      await this.emitter.emit(ChoreographyEventType.PLAYBACK_COMPLETED, {});
    }
  }

  /**
   * NODE_FAILED: Release worker, mark as failed
   */
  private async onNodeFailed(payload: ChoreographyEventPayloads[ChoreographyEventType.NODE_FAILED]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId } = payload;
    const nodesStore = useNodesStore.getState();
    const workersStore = useWorkersStore.getState();

    // Update status
    nodesStore.updateNode(nodeId, { status: "node_failed" });

    // Release worker
    workersStore.releaseWorker(nodeId);

    // Update progress
    useChoreographyStore.getState().incrementFailed();

    // Failed nodes stay in graph (don't trigger dependents)

    // Check if all nodes complete
    if (this.loader.isComplete()) {
      useChoreographyStore.getState().setState(PlaybackState.COMPLETED);
      await this.emitter.emit(ChoreographyEventType.PLAYBACK_COMPLETED, {});
    }
  }

  /**
   * BUDGET_INSUFFICIENT: Node is blocked, waiting for budget reset
   */
  private async onBudgetInsufficient(payload: ChoreographyEventPayloads[ChoreographyEventType.BUDGET_INSUFFICIENT]): Promise<void> {
    if (this.isPaused) return;

    const { nodeId } = payload;
    const nodesStore = useNodesStore.getState();

    // Mark node as blocked (using prioritized as closest match)
    nodesStore.updateNode(nodeId, { status: "node_prioritized" });
  }

  /**
   * BUDGET_RESET: Retry launching blocked nodes
   */
  private async onBudgetReset(payload: ChoreographyEventPayloads[ChoreographyEventType.BUDGET_RESET]): Promise<void> {
    if (this.isPaused) return;

    const { provider } = payload;
    const queueStore = useQueueStore.getState();
    const nodesStore = useNodesStore.getState();

    // Find blocked nodes for this provider
    for (const nodeId of queueStore.priorityQueueNodes) {
      const node = nodesStore.getNode(nodeId);
      const scenarioNode = this.loader.getScenarioNode(nodeId);

      if (
        node?.status === "node_prioritized" &&
        scenarioNode?.provider === provider
      ) {
        // Unblock and retry launch
        nodesStore.updateNode(nodeId, { status: "node_ready" });
        await this.emitter.emit(ChoreographyEventType.NODE_LAUNCHED, { nodeId, provider });
        break; // Only launch one node per reset to avoid overwhelming
      }
    }
  }

  /**
   * Pause all animations
   */
  pause(): void {
    this.isPaused = true;
    this.budgetTracker.pause();
  }

  /**
   * Resume all animations
   */
  resume(): void {
    this.isPaused = false;
    this.budgetTracker.resume();
  }

  /**
   * Cleanup and remove all listeners
   */
  cleanup(): void {
    for (const unsubscribe of this.unsubscribers) {
      unsubscribe();
    }
    this.unsubscribers = [];
    this.pendingAnimations.clear();
  }

  /**
   * Animate with duration
   */
  private async animate(fn: () => Promise<void>): Promise<void> {
    const speed = useChoreographyStore.getState().speed;
    const duration = ANIMATION_DURATION_MS / speed;

    await this.delay(duration);
    if (!this.isPaused) {
      await fn();
    }
  }

  /**
   * Delay helper
   */
  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => {
      const timeout = setTimeout(() => {
        resolve();
      }, ms);

      // Store timeout so we can clear it if paused
      if (this.isPaused) {
        clearTimeout(timeout);
        resolve();
      }
    });
  }

  /**
   * Track pending animation
   */
  private trackAnimation(animation: Promise<void>): void {
    this.pendingAnimations.add(animation);
    animation.finally(() => {
      this.pendingAnimations.delete(animation);
    });
  }

  /**
   * Wait for all pending animations to complete
   */
  async waitForAnimations(): Promise<void> {
    await Promise.all(this.pendingAnimations);
  }
}

// Global singleton
let globalChoreographer: Choreographer | null = null;

/**
 * Get the global choreographer instance
 */
export function getChoreographer(): Choreographer {
  if (!globalChoreographer) {
    globalChoreographer = new Choreographer();
    globalChoreographer.initialize();
  }
  return globalChoreographer;
}

/**
 * Reset the global choreographer
 */
export function resetChoreographer(): void {
  globalChoreographer?.cleanup();
  globalChoreographer = null;
}
