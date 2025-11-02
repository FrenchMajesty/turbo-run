/**
 * Event system for choreography engine
 * Provides type-safe event emitter for orchestrating animations
 */

import { Provider } from "../types/scenario";

export enum ChoreographyEventType {
  // Node lifecycle
  NODE_READY = "NODE_READY",
  NODE_ENQUEUED = "NODE_ENQUEUED",
  PQ_REORDERED = "PQ_REORDERED",
  NODE_LAUNCHED = "NODE_LAUNCHED",
  NODE_DISPATCHED = "NODE_DISPATCHED",
  NODE_ATTEMPT_START = "NODE_ATTEMPT_START",
  NODE_ATTEMPT_COMPLETE = "NODE_ATTEMPT_COMPLETE",
  NODE_COMPLETED = "NODE_COMPLETED",
  NODE_FAILED = "NODE_FAILED",

  // Budget management
  BUDGET_INSUFFICIENT = "BUDGET_INSUFFICIENT",
  BUDGET_RESET = "BUDGET_RESET",

  // Playback control
  PLAYBACK_STARTED = "PLAYBACK_STARTED",
  PLAYBACK_PAUSED = "PLAYBACK_PAUSED",
  PLAYBACK_RESUMED = "PLAYBACK_RESUMED",
  PLAYBACK_RESET = "PLAYBACK_RESET",
  PLAYBACK_COMPLETED = "PLAYBACK_COMPLETED",
}

export interface ChoreographyEventPayloads {
  [ChoreographyEventType.NODE_READY]: { nodeId: string };
  [ChoreographyEventType.NODE_ENQUEUED]: { nodeId: string };
  [ChoreographyEventType.PQ_REORDERED]: { nodeIds: string[] }; // New order
  [ChoreographyEventType.NODE_LAUNCHED]: { nodeId: string; provider: Provider };
  [ChoreographyEventType.NODE_DISPATCHED]: { nodeId: string; workerId: number };
  [ChoreographyEventType.NODE_ATTEMPT_START]: {
    nodeId: string;
    attemptIndex: number;
    durationMs: number;
  };
  [ChoreographyEventType.NODE_ATTEMPT_COMPLETE]: {
    nodeId: string;
    attemptIndex: number;
    success: boolean;
  };
  [ChoreographyEventType.NODE_COMPLETED]: { nodeId: string };
  [ChoreographyEventType.NODE_FAILED]: { nodeId: string };
  [ChoreographyEventType.BUDGET_INSUFFICIENT]: { nodeId: string; provider: Provider };
  [ChoreographyEventType.BUDGET_RESET]: { provider: Provider };
  [ChoreographyEventType.PLAYBACK_STARTED]: Record<string, never>;
  [ChoreographyEventType.PLAYBACK_PAUSED]: Record<string, never>;
  [ChoreographyEventType.PLAYBACK_RESUMED]: Record<string, never>;
  [ChoreographyEventType.PLAYBACK_RESET]: Record<string, never>;
  [ChoreographyEventType.PLAYBACK_COMPLETED]: Record<string, never>;
}

type EventListener<T extends ChoreographyEventType> = (
  payload: ChoreographyEventPayloads[T]
) => void | Promise<void>;

/**
 * Type-safe event emitter for choreography events
 */
export class ChoreographyEventEmitter {
  private listeners: {
    [K in ChoreographyEventType]?: Set<EventListener<K>>;
  } = {};

  /**
   * Register an event listener
   */
  on<T extends ChoreographyEventType>(
    event: T,
    listener: EventListener<T>
  ): () => void {
    if (!this.listeners[event]) {
      this.listeners[event] = new Set() as any;
    }
    (this.listeners[event] as Set<EventListener<any>>).add(listener);

    // Return unsubscribe function
    return () => {
      (this.listeners[event] as Set<EventListener<any>>)?.delete(listener);
    };
  }

  /**
   * Register a one-time event listener
   */
  once<T extends ChoreographyEventType>(
    event: T,
    listener: EventListener<T>
  ): () => void {
    const wrappedListener: EventListener<T> = (payload) => {
      unsubscribe();
      return listener(payload);
    };

    const unsubscribe = this.on(event, wrappedListener);
    return unsubscribe;
  }

  /**
   * Emit an event to all registered listeners
   */
  async emit<T extends ChoreographyEventType>(
    event: T,
    payload: ChoreographyEventPayloads[T]
  ): Promise<void> {
    const listeners = this.listeners[event] as Set<EventListener<T>> | undefined;
    if (!listeners || listeners.size === 0) {
      return;
    }

    // Execute all listeners (supports both sync and async)
    const promises: Promise<void>[] = [];
    listeners.forEach((listener) => {
      const result = listener(payload);
      if (result instanceof Promise) {
        promises.push(result);
      }
    });

    await Promise.all(promises);
  }

  /**
   * Remove all listeners for a specific event, or all events if no event specified
   */
  removeAllListeners(event?: ChoreographyEventType): void {
    if (event) {
      this.listeners[event]?.clear();
    } else {
      this.listeners = {};
    }
  }

  /**
   * Get count of listeners for an event
   */
  listenerCount(event: ChoreographyEventType): number {
    return this.listeners[event]?.size ?? 0;
  }
}

// Global singleton event emitter
let globalEmitter: ChoreographyEventEmitter | null = null;

/**
 * Get the global choreography event emitter
 */
export function getEventEmitter(): ChoreographyEventEmitter {
  if (!globalEmitter) {
    globalEmitter = new ChoreographyEventEmitter();
  }
  return globalEmitter;
}

/**
 * Reset the global event emitter (useful for testing/cleanup)
 */
export function resetEventEmitter(): void {
  globalEmitter?.removeAllListeners();
  globalEmitter = null;
}
