import { useEffect, useState, useCallback, useRef } from 'react';
import { io, Socket } from 'socket.io-client';
import { TurboEvent } from '../types/TurboEvent';
import { Stats } from '../types/Stats';

interface UseWebSocketReturn {
  socket: Socket | null;
  isConnected: boolean;
  prepareGraph: () => void;
  startProcessing: () => void;
  onTurboEvent: (callback: (event: TurboEvent) => void) => void;
  onStatsUpdate: (callback: (stats: Stats) => void) => void;
  onInitialStats: (callback: (stats: Stats) => void) => void;
  onPriorityQueueUpdate: (callback: (nodeIds: string[]) => void) => void;
  onGraphPreparing: (callback: () => void) => void;
  onGraphPrepared: (callback: () => void) => void;
  onGraphCancelled: (callback: () => void) => void;
  onProcessingStarted: (callback: () => void) => void;
  onProcessingCompleted: (callback: () => void) => void;
}

export const useWebSocket = (url: string = 'http://localhost:8081'): UseWebSocketReturn => {
  const [isConnected, setIsConnected] = useState(false);
  const socketRef = useRef<Socket | null>(null);
  const eventCallbackRef = useRef<((event: TurboEvent) => void) | null>(null);
  const statsCallbackRef = useRef<((stats: Stats) => void) | null>(null);
  const initialStatsCallbackRef = useRef<((stats: Stats) => void) | null>(null);
  const priorityQueueCallbackRef = useRef<((nodeIds: string[]) => void) | null>(null);
  const graphPreparingCallbackRef = useRef<(() => void) | null>(null);
  const graphPreparedCallbackRef = useRef<(() => void) | null>(null);
  const graphCancelledCallbackRef = useRef<(() => void) | null>(null);
  const processingStartedCallbackRef = useRef<(() => void) | null>(null);
  const processingCompletedCallbackRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    const socket = io(url, {
      transports: ['websocket', 'polling'],
      upgrade: true,
      rememberUpgrade: true,
    });

    socketRef.current = socket;

    socket.on('connect', () => {
      console.log('✅ Connected to TurboRun server');
      setIsConnected(true);
    });

    socket.on('connect_error', (error) => {
      console.error('❌ Connection error:', error);
      setIsConnected(false);
    });

    socket.on('disconnect', (reason) => {
      console.log('⚠️ Disconnected from server. Reason:', reason);
      setIsConnected(false);
    });

    socket.on('turbo_event', (data: string) => {
      const event: TurboEvent = JSON.parse(data);
      if (eventCallbackRef.current) {
        eventCallbackRef.current(event);
      }
    });

    socket.on('stats_update', (data: string) => {
      const stats: Stats = JSON.parse(data);
      if (statsCallbackRef.current) {
        statsCallbackRef.current(stats);
      }
      // Also trigger priority queue update if snapshot is present
      if (stats.PriorityQueueSnapshot && priorityQueueCallbackRef.current) {
        priorityQueueCallbackRef.current(stats.PriorityQueueSnapshot);
      }
    });

    socket.on('initial_stats', (data: string) => {
      const stats: Stats = JSON.parse(data);
      if (initialStatsCallbackRef.current) {
        initialStatsCallbackRef.current(stats);
      }
      // Also trigger priority queue update if snapshot is present
      if (stats.PriorityQueueSnapshot && priorityQueueCallbackRef.current) {
        priorityQueueCallbackRef.current(stats.PriorityQueueSnapshot);
      }
    });

    socket.on('graph_preparing', () => {
      console.log('Graph preparing');
      if (graphPreparingCallbackRef.current) {
        graphPreparingCallbackRef.current();
      }
    });

    socket.on('graph_prepared', () => {
      console.log('Graph prepared');
      if (graphPreparedCallbackRef.current) {
        graphPreparedCallbackRef.current();
      }
    });

    socket.on('graph_cancelled', () => {
      console.log('Graph cancelled');
      if (graphCancelledCallbackRef.current) {
        graphCancelledCallbackRef.current();
      }
    });

    socket.on('processing_started', () => {
      console.log('Processing started');
      if (processingStartedCallbackRef.current) {
        processingStartedCallbackRef.current();
      }
    });

    socket.on('processing_completed', () => {
      console.log('Processing completed');
      if (processingCompletedCallbackRef.current) {
        processingCompletedCallbackRef.current();
      }
    });

    return () => {
      socket.disconnect();
    };
  }, [url]);

  const prepareGraph = useCallback(() => {
    if (socketRef.current && isConnected) {
      console.log('Prepare button clicked, emitting prepare_graph event');
      socketRef.current.emit('prepare_graph');
    }
  }, [isConnected]);

  const startProcessing = useCallback(() => {
    if (socketRef.current && isConnected) {
      console.log('Start button clicked, emitting start_processing event');
      socketRef.current.emit('start_processing');
    }
  }, [isConnected]);

  const onTurboEvent = useCallback((callback: (event: TurboEvent) => void) => {
    eventCallbackRef.current = callback;
  }, []);

  const onStatsUpdate = useCallback((callback: (stats: Stats) => void) => {
    statsCallbackRef.current = callback;
  }, []);

  const onInitialStats = useCallback((callback: (stats: Stats) => void) => {
    initialStatsCallbackRef.current = callback;
  }, []);

  const onGraphPreparing = useCallback((callback: () => void) => {
    graphPreparingCallbackRef.current = callback;
  }, []);

  const onGraphPrepared = useCallback((callback: () => void) => {
    graphPreparedCallbackRef.current = callback;
  }, []);

  const onGraphCancelled = useCallback((callback: () => void) => {
    graphCancelledCallbackRef.current = callback;
  }, []);

  const onProcessingStarted = useCallback((callback: () => void) => {
    processingStartedCallbackRef.current = callback;
  }, []);

  const onProcessingCompleted = useCallback((callback: () => void) => {
    processingCompletedCallbackRef.current = callback;
  }, []);

  const onPriorityQueueUpdate = useCallback((callback: (nodeIds: string[]) => void) => {
    priorityQueueCallbackRef.current = callback;
  }, []);

  return {
    socket: socketRef.current,
    isConnected,
    prepareGraph,
    startProcessing,
    onTurboEvent,
    onStatsUpdate,
    onInitialStats,
    onPriorityQueueUpdate,
    onGraphPreparing,
    onGraphPrepared,
    onGraphCancelled,
    onProcessingStarted,
    onProcessingCompleted,
  };
};
