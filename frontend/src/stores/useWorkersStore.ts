import { create } from 'zustand';

interface WorkersState {
  workerStates: { [workerId: string]: string }; // Maps worker ID to node ID (empty if idle)
  assignWorker: (workerId: number, nodeId: string) => void;
  releaseWorker: (nodeId: string) => void;
  clearWorkers: () => void;
}

export const useWorkersStore = create<WorkersState>((set) => ({
  workerStates: {},

  assignWorker: (workerId, nodeId) =>
    set((state) => ({
      workerStates: {
        ...state.workerStates,
        [workerId.toString()]: nodeId,
      },
    })),

  releaseWorker: (nodeId) =>
    set((state) => {
      const newStates = { ...state.workerStates };
      // Find and clear the worker that was working on this node
      for (const [workerId, workerNodeId] of Object.entries(newStates)) {
        if (workerNodeId === nodeId) {
          delete newStates[workerId];
          break;
        }
      }
      return { workerStates: newStates };
    }),

  clearWorkers: () => set({ workerStates: {} }),
}));
