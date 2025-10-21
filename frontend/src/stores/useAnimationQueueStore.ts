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

export interface ConsumerQueueItem {
  nodeId: string;
  eventType: string;
  animationId: string; // Reference to the animation in global map
}

interface AnimationQueueState {
  // Global map: nodeId -> queue of animations for that node
  // Maintains insertion order (first animation must complete before next)
  globalAnimationMap: Map<string, QueuedAnimation[]>;

  // Per-component consumer queues (lightweight refs to animations)
  graphConsumer: ConsumerQueueItem[];
  queueConsumer: ConsumerQueueItem[];
  gridConsumer: ConsumerQueueItem[];

  // Actions
  enqueueAnimation: (animation: QueuedAnimation) => void;
  dequeueAnimation: (nodeId: string) => void;
  getNextAnimation: (nodeId: string) => QueuedAnimation | undefined;
  clearQueues: () => void;
}

export const useAnimationQueueStore = create<AnimationQueueState>((set, get) => ({
  globalAnimationMap: new Map(),
  graphConsumer: [],
  queueConsumer: [],
  gridConsumer: [],

  enqueueAnimation: (animation) => {
    const { nodeId, component, event } = animation;

    set((state) => {
      // Add to global map
      const newMap = new Map(state.globalAnimationMap);
      const nodeAnimations = newMap.get(nodeId) || [];
      newMap.set(nodeId, [...nodeAnimations, animation]);

      // Add to appropriate consumer queue
      const consumerItem: ConsumerQueueItem = {
        nodeId,
        eventType: event.type,
        animationId: animation.id,
      };

      let updates: Partial<AnimationQueueState> = { globalAnimationMap: newMap };

      switch (component) {
        case 'graph':
          updates.graphConsumer = [...state.graphConsumer, consumerItem];
          break;
        case 'priority_queue':
          updates.queueConsumer = [...state.queueConsumer, consumerItem];
          break;
        case 'worker_grid':
          updates.gridConsumer = [...state.gridConsumer, consumerItem];
          break;
      }

      return updates;
    });
  },

  dequeueAnimation: (nodeId) => {
    set((state) => {
      const newMap = new Map(state.globalAnimationMap);
      const nodeAnimations = newMap.get(nodeId);

      if (nodeAnimations && nodeAnimations.length > 0) {
        const remaining = nodeAnimations.slice(1);
        if (remaining.length === 0) {
          newMap.delete(nodeId);
        } else {
          newMap.set(nodeId, remaining);
        }
      }

      return { globalAnimationMap: newMap };
    });
  },

  getNextAnimation: (nodeId) => {
    const animations = get().globalAnimationMap.get(nodeId);
    return animations?.[0];
  },

  clearQueues: () => {
    set({
      globalAnimationMap: new Map(),
      graphConsumer: [],
      queueConsumer: [],
      gridConsumer: [],
    });
  },
}));
