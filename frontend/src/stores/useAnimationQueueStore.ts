import { create } from 'zustand';
import { TurboEvent } from '../types/TurboEvent';

export type ComponentType = 'graph' | 'priority_queue' | 'worker_grid';

export interface QueuedAnimation {
  id: string; // Unique ID for this animation
  nodeId: string;
  event: TurboEvent;
  component: ComponentType;
  timestamp: number;
  duration: number; // in milliseconds
}

interface AnimationQueueState {
  // Queues per component - simple arrays that hold animations
  graphQueue: QueuedAnimation[];
  priorityQueueQueue: QueuedAnimation[];
  workerGridQueue: QueuedAnimation[];

  // Currently animating items (tracked for visibility, not for logic)
  graphAnimating: Map<string, QueuedAnimation>;
  priorityQueueAnimating: Map<string, QueuedAnimation>;
  workerGridAnimating: Map<string, QueuedAnimation>;

  // Actions
  enqueueAnimation: (animation: QueuedAnimation) => void;
  clearQueues: () => void;
}

export const useAnimationQueueStore = create<AnimationQueueState>((set, get) => ({
  graphQueue: [],
  priorityQueueQueue: [],
  workerGridQueue: [],
  graphAnimating: new Map(),
  priorityQueueAnimating: new Map(),
  workerGridAnimating: new Map(),

  enqueueAnimation: (animation) => {
    const { component } = animation;

    set((state) => {
      switch (component) {
        case 'graph':
          return { graphQueue: [...state.graphQueue, animation] };
        case 'priority_queue':
          return { priorityQueueQueue: [...state.priorityQueueQueue, animation] };
        case 'worker_grid':
          return { workerGridQueue: [...state.workerGridQueue, animation] };
        default:
          return state;
      }
    });
  },

  clearQueues: () => {
    set({
      graphQueue: [],
      priorityQueueQueue: [],
      workerGridQueue: [],
      graphAnimating: new Map(),
      priorityQueueAnimating: new Map(),
      workerGridAnimating: new Map(),
    });
  },
}));
