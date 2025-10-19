import { create } from 'zustand';
import { io, Socket } from 'socket.io-client';

interface WebSocketState {
  socket: Socket | null;
  isConnected: boolean;
  initialize: (url?: string) => void;
  disconnect: () => void;
  prepareGraph: () => void;
  startProcessing: () => void;
}

export const useWebSocketStore = create<WebSocketState>((set, get) => ({
  socket: null,
  isConnected: false,

  initialize: (url = 'http://localhost:8081') => {
    const socket = io(url, {
      transports: ['websocket', 'polling'],
      upgrade: true,
      rememberUpgrade: true,
    });

    socket.on('connect', () => {
      console.log('✅ Connected to TurboRun server');
      set({ isConnected: true });
    });

    socket.on('connect_error', (error) => {
      console.error('❌ Connection error:', error);
      set({ isConnected: false });
    });

    socket.on('disconnect', (reason) => {
      console.log('⚠️ Disconnected from server. Reason:', reason);
      set({ isConnected: false });
    });

    set({ socket });
  },

  disconnect: () => {
    const { socket } = get();
    if (socket) {
      socket.disconnect();
      set({ socket: null, isConnected: false });
    }
  },

  prepareGraph: () => {
    const { socket, isConnected } = get();
    if (socket && isConnected) {
      console.log('Prepare button clicked, emitting prepare_graph event');
      socket.emit('prepare_graph');
    }
  },

  startProcessing: () => {
    const { socket, isConnected } = get();
    if (socket && isConnected) {
      console.log('Start button clicked, emitting start_processing event');
      socket.emit('start_processing');
    }
  },
}));
