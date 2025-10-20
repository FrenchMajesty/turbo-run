import { useEffect } from 'react';
import { useWebSocketStore } from '../stores/useWebSocketStore';
import { useNodesStore } from '../stores/useNodesStore';
import { useWorkersStore } from '../stores/useWorkersStore';
import { useQueueStore } from '../stores/useQueueStore';
import { useStatsStore } from '../stores/useStatsStore';
import { useUIStore } from '../stores/useUIStore';
import { useEventsStore } from '../stores/useEventsStore';
import { useAnimationQueueStore, ComponentType, QueuedAnimation } from '../stores/useAnimationQueueStore';
import { TurboEvent, EventType } from '../types/TurboEvent';
import { Stats } from '../types/Stats';

const DEFAULT_ANIMATION_DURATION = 300; // ms

/**
 * Maps event types to their target component for animation
 */
const getComponentForEvent = (eventType: EventType): ComponentType => {
  switch (eventType) {
    case 'priority_queue_add':
    case 'priority_queue_remove':
      return 'priority_queue';

    case 'node_running':
    case 'node_completed':
    case 'node_failed':
      return 'worker_grid';

    case 'node_created':
    case 'node_ready':
    case 'node_prioritized':
    case 'node_dispatched':
    case 'node_retrying':
    default:
      return 'graph';
  }
};

export const useWebSocketSync = () => {
  const socket = useWebSocketStore((state) => state.socket);

  const { clearNodes } = useNodesStore();
  const { clearWorkers } = useWorkersStore();
  const { setQueue, clearQueue } = useQueueStore();
  const { setStats } = useStatsStore();
  const { setIsPreparing, setIsGraphPrepared, setIsProcessing, resetUI } = useUIStore();
  const { addEvent, clearEvents } = useEventsStore();
  const { enqueueAnimation, clearQueues } = useAnimationQueueStore();

  useEffect(() => {
    if (!socket) return;

    // Handle turbo events (node lifecycle)
    const handleTurboEvent = (data: string) => {
      const event: TurboEvent = JSON.parse(data);
      const nodeId = event.node_id;
      const type = event.type;

      // Add to event log (immediate, no animation needed)
      addEvent(event);

      // Route event through animation queue instead of direct store updates
      const component = getComponentForEvent(type);

      const queuedAnimation: QueuedAnimation = {
        id: `${nodeId}-${type}-${Date.now()}`, // Unique animation ID
        nodeId,
        event,
        component,
        timestamp: Date.now(),
        duration: DEFAULT_ANIMATION_DURATION,
      };

      enqueueAnimation(queuedAnimation);
    };

    // Handle stats updates
    const handleStatsUpdate = (data: string) => {
      const stats: Stats = JSON.parse(data);
      setStats(stats);

      // Note: We no longer update PriorityQueue from stats snapshots
      // The PQ is managed exclusively through turbo_events (priority_queue_add/remove)
      // This ensures all updates go through the animation pipeline
    };

    // Handle initial stats
    const handleInitialStats = (data: string) => {
      const stats: Stats = JSON.parse(data);
      console.log('Initial stats:', stats);
      setStats(stats);

      // Note: We no longer update PriorityQueue from stats snapshots
      // The PQ is managed exclusively through turbo_events (priority_queue_add/remove)
      // This ensures all updates go through the animation pipeline
    };

    // Handle graph lifecycle events
    const handleGraphPreparing = () => {
      console.log('Graph preparing');
      setIsPreparing(true);
      setIsGraphPrepared(false);
    };

    const handleGraphPrepared = () => {
      console.log('Graph prepared');
      setIsPreparing(false);
      setIsGraphPrepared(true);
    };

    const handleGraphCancelled = () => {
      console.log('Graph cancelled');
      resetUI();
      clearNodes();
      clearQueue();
      clearWorkers();
      clearEvents();
      clearQueues(); // Clear animation queues as well
    };

    const handleProcessingStarted = () => {
      console.log('Processing started');
      setIsProcessing(true);
    };

    const handleProcessingCompleted = () => {
      console.log('Processing completed');
      setIsProcessing(false);
      setIsGraphPrepared(false);
    };

    // Register socket event handlers
    socket.on('turbo_event', handleTurboEvent);
    socket.on('stats_update', handleStatsUpdate);
    socket.on('initial_stats', handleInitialStats);
    socket.on('graph_preparing', handleGraphPreparing);
    socket.on('graph_prepared', handleGraphPrepared);
    socket.on('graph_cancelled', handleGraphCancelled);
    socket.on('processing_started', handleProcessingStarted);
    socket.on('processing_completed', handleProcessingCompleted);

    // Cleanup event listeners on unmount
    return () => {
      socket.off('turbo_event', handleTurboEvent);
      socket.off('stats_update', handleStatsUpdate);
      socket.off('initial_stats', handleInitialStats);
      socket.off('graph_preparing', handleGraphPreparing);
      socket.off('graph_prepared', handleGraphPrepared);
      socket.off('graph_cancelled', handleGraphCancelled);
      socket.off('processing_started', handleProcessingStarted);
      socket.off('processing_completed', handleProcessingCompleted);
    };
  }, [socket, clearNodes, clearWorkers, setQueue, clearQueue, setStats, setIsPreparing, setIsGraphPrepared, setIsProcessing, resetUI, addEvent, clearEvents, enqueueAnimation, clearQueues]);
};
