import { useEffect } from 'react';
import { useWebSocketStore } from '../stores/useWebSocketStore';
import { useNodesStore } from '../stores/useNodesStore';
import { useWorkersStore } from '../stores/useWorkersStore';
import { useQueueStore } from '../stores/useQueueStore';
import { useStatsStore } from '../stores/useStatsStore';
import { useUIStore } from '../stores/useUIStore';
import { useEventsStore } from '../stores/useEventsStore';
import { TurboEvent } from '../types/TurboEvent';
import { Stats } from '../types/Stats';

export const useWebSocketSync = () => {
  const socket = useWebSocketStore((state) => state.socket);

  const { addNode, updateNode, deleteNode, clearNodes } = useNodesStore();
  const { assignWorker, releaseWorker, clearWorkers } = useWorkersStore();
  const { addToQueue, removeFromQueue, setQueue, clearQueue } = useQueueStore();
  const { setStats } = useStatsStore();
  const { setIsPreparing, setIsGraphPrepared, setIsProcessing, resetUI } = useUIStore();
  const { addEvent, clearEvents } = useEventsStore();

  useEffect(() => {
    if (!socket) return;

    // Handle turbo events (node lifecycle)
    const handleTurboEvent = (data: string) => {
      const event: TurboEvent = JSON.parse(data);
      const nodeId = event.node_id;
      const type = event.type;
      const eventData = event.data || {};

      // Add to event log
      addEvent(event);

      // Handle priority queue events
      if (type === 'priority_queue_add') {
        addToQueue(nodeId);
        return;
      }

      if (type === 'priority_queue_remove') {
        removeFromQueue(nodeId);
        return;
      }

      // Handle worker state changes
      if (type === 'node_running' && eventData.worker_id !== undefined) {
        assignWorker(eventData.worker_id, nodeId);
      }

      // Clear worker state when node completes or fails
      if (type === 'node_completed' || type === 'node_failed') {
        releaseWorker(nodeId);
      }

      // Update or add node
      const existingNode = useNodesStore.getState().getNode(nodeId);
      if (!existingNode) {
        addNode(nodeId, {
          id: nodeId,
          status: type,
          dependencies: eventData.dependencies || [],
          provider: eventData.provider || 'unknown',
          tokens: eventData.estimated_tokens || 0,
          data: eventData,
        });
      } else {
        updateNode(nodeId, {
          status: type,
          data: eventData,
        });
      }

      // Auto-remove completed nodes after 3 seconds
      if (type === 'node_completed') {
        setTimeout(() => {
          deleteNode(nodeId);
        }, 3000);
      }
    };

    // Handle stats updates
    const handleStatsUpdate = (data: string) => {
      const stats: Stats = JSON.parse(data);
      setStats(stats);

      // Update priority queue if snapshot is present
      if (stats.PriorityQueueSnapshot) {
        setQueue(stats.PriorityQueueSnapshot);
      }
    };

    // Handle initial stats
    const handleInitialStats = (data: string) => {
      const stats: Stats = JSON.parse(data);
      console.log('Initial stats:', stats);
      setStats(stats);

      // Update priority queue if snapshot is present
      if (stats.PriorityQueueSnapshot) {
        setQueue(stats.PriorityQueueSnapshot);
      }
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
  }, [socket, addNode, updateNode, deleteNode, clearNodes, assignWorker, releaseWorker, clearWorkers, addToQueue, removeFromQueue, setQueue, clearQueue, setStats, setIsPreparing, setIsGraphPrepared, setIsProcessing, resetUI, addEvent, clearEvents]);
};
