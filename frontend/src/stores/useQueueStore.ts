import { create } from 'zustand';

interface QueueState {
  priorityQueueNodes: string[];
  addToQueue: (nodeId: string) => void;
  removeFromQueue: (nodeId: string) => void;
  setQueue: (nodeIds: string[]) => void;
  clearQueue: () => void;
}

export const useQueueStore = create<QueueState>((set) => ({
  priorityQueueNodes: [],

  addToQueue: (nodeId) =>
    set((state) => ({
      priorityQueueNodes: [...state.priorityQueueNodes, nodeId],
    })),

  removeFromQueue: (nodeId) =>
    set((state) => ({
      priorityQueueNodes: state.priorityQueueNodes.filter((id) => id !== nodeId),
    })),

  setQueue: (nodeIds) => set({ priorityQueueNodes: nodeIds }),

  clearQueue: () => set({ priorityQueueNodes: [] }),
}));
