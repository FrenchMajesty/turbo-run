import { useEffect, useRef, useCallback } from 'react';
import { useAnimationQueueStore, QueuedAnimation, ConsumerQueueItem } from '../stores/useAnimationQueueStore';
import { useNodesStore } from '../stores/useNodesStore';
import { useQueueStore } from '../stores/useQueueStore';
import { useWorkersStore } from '../stores/useWorkersStore';
import { useRateLimitedQueue } from './useRateLimitedQueue';

// Rate limits per component (animations per second)
const RATE_LIMITS = {
  graph: 10,           // Fast for many nodes
  priority_queue: 10,  // Moderate for slide animations
  worker_grid: 10,     // Moderate-fast for fade animations
};

/**
 * Animation Orchestrator Hook
 *
 * New architecture:
 * 1. Global animation map tracks all pending animations per node
 * 2. Consumer queues (graph/queue/grid) hold references to animations
 * 3. When processing, check if animation is first in global map for that node
 * 4. Only process if no other animation is active for that node
 * 5. Rate-limited processing per component
 */
export const useAnimationOrchestrator = () => {
  const {
    graphConsumer,
    queueConsumer,
    gridConsumer,
    getNextAnimation,
    dequeueAnimation,
  } = useAnimationQueueStore();

  const { addNode, updateNode, deleteNode } = useNodesStore();
  const { addToQueue, removeFromQueue } = useQueueStore();
  const { assignWorker, releaseWorker } = useWorkersStore();

  const animationTimersRef = useRef<Map<string, NodeJS.Timeout>>(new Map());
  const processedConsumerItemsRef = useRef<Set<string>>(new Set());

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
   * Try to process a consumer item
   * Returns true if processed, false if blocked (node has pending animation)
   */
  const tryProcessConsumerItem = useCallback((item: ConsumerQueueItem): boolean => {
    const { nodeId, animationId } = item;

    // Get the next animation for this node from global map
    const nextAnimation = getNextAnimation(nodeId);

    // Check if this consumer item matches the next animation for this node
    if (!nextAnimation || nextAnimation.id !== animationId) {
      // Another animation is ahead of us for this node, skip for now
      return false;
    }

    // We can process! Execute the store update
    executeStoreUpdate(nextAnimation);

    // Schedule completion: remove from global map after animation duration
    const timer = setTimeout(() => {
      dequeueAnimation(nodeId);
      animationTimersRef.current.delete(nextAnimation.id);
    }, nextAnimation.duration);

    animationTimersRef.current.set(nextAnimation.id, timer);

    return true;
  }, [getNextAnimation, executeStoreUpdate, dequeueAnimation]);

  /**
   * Process consumer items with blocking logic
   * Only processes if the animation is first for that node
   */
  const processConsumerItem = useCallback((item: ConsumerQueueItem) => {
    // Skip if already processed
    if (processedConsumerItemsRef.current.has(item.animationId)) {
      return;
    }

    // Try to process (might be blocked by another animation for same node)
    const processed = tryProcessConsumerItem(item);

    if (processed) {
      processedConsumerItemsRef.current.add(item.animationId);
    } else {
      // Blocked - retry after a short delay (reactive approach)
      setTimeout(() => {
        if (!processedConsumerItemsRef.current.has(item.animationId)) {
          processConsumerItem(item);
        }
      }, 50); // Check again in 50ms
    }
  }, [tryProcessConsumerItem]);

  // Set up rate-limited queues for each component
  const graphRateLimitedQueue = useRateLimitedQueue(
    { maxPerSecond: RATE_LIMITS.graph },
    processConsumerItem
  );

  const priorityQueueRateLimitedQueue = useRateLimitedQueue(
    { maxPerSecond: RATE_LIMITS.priority_queue },
    processConsumerItem
  );

  const workerGridRateLimitedQueue = useRateLimitedQueue(
    { maxPerSecond: RATE_LIMITS.worker_grid },
    processConsumerItem
  );

  /**
   * Monitor consumer queues and feed new items into rate-limited queues
   */
  const graphProcessedRef = useRef<Set<string>>(new Set());
  const pqProcessedRef = useRef<Set<string>>(new Set());
  const wgProcessedRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    graphConsumer.forEach((item) => {
      if (!graphProcessedRef.current.has(item.animationId)) {
        graphRateLimitedQueue.enqueue(item);
        graphProcessedRef.current.add(item.animationId);
      }
    });
  }, [graphConsumer, graphRateLimitedQueue]);

  useEffect(() => {
    queueConsumer.forEach((item) => {
      if (!pqProcessedRef.current.has(item.animationId)) {
        priorityQueueRateLimitedQueue.enqueue(item);
        pqProcessedRef.current.add(item.animationId);
      }
    });
  }, [queueConsumer, priorityQueueRateLimitedQueue]);

  useEffect(() => {
    gridConsumer.forEach((item) => {
      if (!wgProcessedRef.current.has(item.animationId)) {
        workerGridRateLimitedQueue.enqueue(item);
        wgProcessedRef.current.add(item.animationId);
      }
    });
  }, [gridConsumer, workerGridRateLimitedQueue]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      animationTimersRef.current.forEach((timer) => clearTimeout(timer));
      animationTimersRef.current.clear();
      graphProcessedRef.current.clear();
      pqProcessedRef.current.clear();
      wgProcessedRef.current.clear();
      processedConsumerItemsRef.current.clear();
    };
  }, []);

  return null; // This hook doesn't return anything, it just orchestrates in the background
};
