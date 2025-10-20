import { useEffect, useRef, useCallback } from 'react';
import { useAnimationQueueStore, QueuedAnimation } from '../stores/useAnimationQueueStore';
import { useNodesStore } from '../stores/useNodesStore';
import { useQueueStore } from '../stores/useQueueStore';
import { useWorkersStore } from '../stores/useWorkersStore';
import { useRateLimitedQueue } from './useRateLimitedQueue';

// Rate limits per component (animations per second)
const RATE_LIMITS = {
  graph: 20000,           // Fast for many nodes
  priority_queue: 20000,  // Moderate for slide animations
  worker_grid: 20000,     // Moderate-fast for fade animations
};

/**
 * Animation Orchestrator Hook
 *
 * This hook processes animation queues at a controlled rate:
 * 1. Monitors the animation queue store for new animations
 * 2. Feeds them into rate-limited queues (one per component)
 * 3. Processes animations sequentially at the configured rate
 * 4. Updates stores and schedules completion after animation duration
 */
export const useAnimationOrchestrator = () => {
  const {
    graphQueue,
    priorityQueueQueue,
    workerGridQueue,
  } = useAnimationQueueStore();

  const { addNode, updateNode, deleteNode } = useNodesStore();
  const { addToQueue, removeFromQueue } = useQueueStore();
  const { assignWorker, releaseWorker } = useWorkersStore();

  const animationTimersRef = useRef<Map<string, NodeJS.Timeout>>(new Map());

  /**
   * Execute the actual store update based on the event type
   * This is memoized to prevent recreating on every render
   */
  const executeStoreUpdate = useCallback((animation: QueuedAnimation) => {
    const { event, nodeId } = animation;
    const eventType = event.type;
    const eventData = event.data || {};
    console.log('executeStoreUpdate()', eventType, nodeId, new Date().toISOString());

    switch (eventType) {
      // Graph events - add/update nodes
      case 'node_created':
      case 'node_ready':
      case 'node_prioritized':
      case 'node_dispatched':
      case 'node_running':
      case 'node_retrying':
      case 'node_completed':
      case 'node_failed': {
        const existingNode = useNodesStore.getState().getNode(nodeId);
        if (!existingNode) {
          addNode(nodeId, {
            id: nodeId,
            status: eventType,
            dependencies: eventData.dependencies || [],
            provider: eventData.provider || 'unknown',
            tokens: eventData.estimated_tokens || 0,
            data: eventData,
          });
        } else {
          updateNode(nodeId, {
            status: eventType,
            data: eventData,
          });
        }

        // Auto-remove completed nodes after animation + delay
        if (eventType === 'node_completed') {
          setTimeout(() => {
            deleteNode(nodeId);
          }, 3000);
        }
        break;
      }

      // Priority queue events
      case 'priority_queue_add':
        addToQueue(nodeId);
        break;

      case 'priority_queue_remove':
        removeFromQueue(nodeId);
        break;

      default:
        break;
    }

    // Worker assignment (handled separately from event type)
    if (eventType === 'node_running' && eventData.worker_id !== undefined) {
      assignWorker(eventData.worker_id, nodeId);
    }

    // Clear worker state when node completes or fails
    if (eventType === 'node_completed' || eventType === 'node_failed') {
      releaseWorker(nodeId);
    }
  }, [addNode, updateNode, deleteNode, addToQueue, removeFromQueue, assignWorker, releaseWorker]);

  /**
   * Process a single animation
   * This is memoized with executeStoreUpdate as dependency
   */
  const processAnimation = useCallback((animation: QueuedAnimation) => {
    // Execute the store update to trigger visual animation
    executeStoreUpdate(animation);

    // Schedule completion timer
    const timer = setTimeout(() => {
      animationTimersRef.current.delete(animation.id);
    }, animation.duration);

    animationTimersRef.current.set(animation.id, timer);
  }, [executeStoreUpdate]);

  // Set up rate-limited queues for each component
  // These are stable because processAnimation is memoized
  const graphRateLimitedQueue = useRateLimitedQueue(
    { maxPerSecond: RATE_LIMITS.graph },
    processAnimation
  );

  const priorityQueueRateLimitedQueue = useRateLimitedQueue(
    { maxPerSecond: RATE_LIMITS.priority_queue },
    processAnimation
  );

  const workerGridRateLimitedQueue = useRateLimitedQueue(
    { maxPerSecond: RATE_LIMITS.worker_grid },
    processAnimation
  );

  /**
   * Monitor animation queue store and feed new animations into rate-limited queues
   * Use refs to track what we've already enqueued to avoid duplicates
   */
  const graphProcessedRef = useRef<Set<string>>(new Set());
  const pqProcessedRef = useRef<Set<string>>(new Set());
  const wgProcessedRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    graphQueue.forEach((animation) => {
      if (!graphProcessedRef.current.has(animation.id)) {
        graphRateLimitedQueue.enqueue(animation);
        graphProcessedRef.current.add(animation.id);
      }
    });
  }, [graphQueue]); // Only depend on graphQueue, not the hook object

  useEffect(() => {
    priorityQueueQueue.forEach((animation) => {
      if (!pqProcessedRef.current.has(animation.id)) {
        priorityQueueRateLimitedQueue.enqueue(animation);
        pqProcessedRef.current.add(animation.id);
      }
    });
  }, [priorityQueueQueue]); // Only depend on priorityQueueQueue, not the hook object

  useEffect(() => {
    workerGridQueue.forEach((animation) => {
      if (!wgProcessedRef.current.has(animation.id)) {
        workerGridRateLimitedQueue.enqueue(animation);
        wgProcessedRef.current.add(animation.id);
      }
    });
  }, [workerGridQueue]); // Only depend on workerGridQueue, not the hook object

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      animationTimersRef.current.forEach((timer) => clearTimeout(timer));
      animationTimersRef.current.clear();
      graphProcessedRef.current.clear();
      pqProcessedRef.current.clear();
      wgProcessedRef.current.clear();
    };
  }, []);

  return null; // This hook doesn't return anything, it just orchestrates in the background
};
