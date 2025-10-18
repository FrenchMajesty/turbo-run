import { useEffect, useState, useCallback, useRef } from 'react';
import { io, Socket } from 'socket.io-client';
import { TurboEvent } from '../types/TurboEvent';
import { Stats } from '../types/Stats';

interface UseWebSocketReturn {
  socket: Socket | null;
  isConnected: boolean;
  startProcessing: () => void;
  onTurboEvent: (callback: (event: TurboEvent) => void) => void;
  onStatsUpdate: (callback: (stats: Stats) => void) => void;
  onInitialStats: (callback: (stats: Stats) => void) => void;
  onProcessingStarted: (callback: () => void) => void;
}

export const useWebSocket = (url: string = 'http://localhost:8081'): UseWebSocketReturn => {
  const [isConnected, setIsConnected] = useState(false);
  const socketRef = useRef<Socket | null>(null);
  const eventCallbackRef = useRef<((event: TurboEvent) => void) | null>(null);
  const statsCallbackRef = useRef<((stats: Stats) => void) | null>(null);
  const initialStatsCallbackRef = useRef<((stats: Stats) => void) | null>(null);
  const processingStartedCallbackRef = useRef<(() => void) | null>(null);

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
    });

    socket.on('initial_stats', (data: string) => {
      const stats: Stats = JSON.parse(data);
      if (initialStatsCallbackRef.current) {
        initialStatsCallbackRef.current(stats);
      }
    });

    socket.on('processing_started', () => {
      console.log('Processing started');
      if (processingStartedCallbackRef.current) {
        processingStartedCallbackRef.current();
      }
    });

    return () => {
      socket.disconnect();
    };
  }, [url]);

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

  const onProcessingStarted = useCallback((callback: () => void) => {
    processingStartedCallbackRef.current = callback;
  }, []);

  return {
    socket: socketRef.current,
    isConnected,
    startProcessing,
    onTurboEvent,
    onStatsUpdate,
    onInitialStats,
    onProcessingStarted,
  };
};
