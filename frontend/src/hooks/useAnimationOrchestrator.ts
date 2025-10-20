import { useEffect, useRef } from 'react';
import { useAnimationQueueStore, ComponentType, QueuedAnimation } from '../stores/useAnimationQueueStore';
import { useNodesStore } from '../stores/useNodesStore';
import { useQueueStore } from '../stores/useQueueStore';
import { useWorkersStore } from '../stores/useWorkersStore';

const DEFAULT_ANIMATION_DURATION = 300; // ms

/**
 * Animation Orchestrator Hook
 *
 * This hook runs a continuous loop that:
 * 1. Checks each component's animation queue for ready animations
 * 2. Starts animations when slots are available and node dependencies are satisfied
 * 3. Updates stores at the appropriate time to trigger visual animations
 * 4. Completes animations after duration expires
 */
export const useAnimationOrchestrator = () => {
  const {
    getNextReadyAnimation,
    startAnimation,
    completeAnimation,
    graphQueue,
    priorityQueueQueue,
    workerGridQueue,
  } = useAnimationQueueStore();

  const { addNode, updateNode, deleteNode } = useNodesStore();
  const { addToQueue, removeFromQueue } = useQueueStore();
  const { assignWorker, releaseWorker } = useWorkersStore();

  const animationTimersRef = useRef<Map<string, NodeJS.Timeout>>(new Map());

  // Process animation queues continuously
  useEffect(() => {
    const processQueue = (component: ComponentType) => {
      const nextAnimation = getNextReadyAnimation(component);

      if (!nextAnimation) return;

      // Start the animation
      startAnimation(nextAnimation.id, component);

      // Execute the corresponding store update to trigger visual animation
      executeStoreUpdate(nextAnimation);

      // Schedule completion after duration
      const timer = setTimeout(() => {
        completeAnimation(nextAnimation.id, component);
        animationTimersRef.current.delete(nextAnimation.id);
      }, nextAnimation.duration);

      animationTimersRef.current.set(nextAnimation.id, timer);
    };

    // Process all component queues on a regular interval
    const intervalId = setInterval(() => {
      processQueue('graph');
      processQueue('priority_queue');
      processQueue('worker_grid');
    }, 50); // Check every 50ms for smooth processing

    return () => {
      clearInterval(intervalId);
      // Clear any pending timers
      animationTimersRef.current.forEach((timer) => clearTimeout(timer));
      animationTimersRef.current.clear();
    };
  }, [graphQueue, priorityQueueQueue, workerGridQueue, getNextReadyAnimation, startAnimation, completeAnimation]);

  /**
   * Execute the actual store update based on the event type
   * This triggers the component to re-render with the new data,
   * which Framer Motion will then animate
   */
  const executeStoreUpdate = (animation: QueuedAnimation) => {
    const { event, nodeId } = animation;
    const eventType = event.type;
    const eventData = event.data || {};

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
  };

  return null; // This hook doesn't return anything, it just orchestrates in the background
};
