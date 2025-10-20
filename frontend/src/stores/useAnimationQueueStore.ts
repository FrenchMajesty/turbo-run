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

export interface AnimationState {
  status: 'queued' | 'animating' | 'completed';
  startTime?: number;
  endTime?: number;
}

interface AnimationQueueState {
  // Queues per component
  graphQueue: QueuedAnimation[];
  priorityQueueQueue: QueuedAnimation[];
  workerGridQueue: QueuedAnimation[];

  // Currently animating items (max 10 per component)
  graphAnimating: Map<string, AnimationState>; // animationId -> state
  priorityQueueAnimating: Map<string, AnimationState>;
  workerGridAnimating: Map<string, AnimationState>;

  // Track which nodes are currently animating (for dependency enforcement)
  nodeAnimationStatus: Map<string, { component: ComponentType; animationId: string }>; // nodeId -> current animation

  // Actions
  enqueueAnimation: (animation: QueuedAnimation) => void;
  startAnimation: (animationId: string, component: ComponentType) => void;
  completeAnimation: (animationId: string, component: ComponentType) => void;
  getNextReadyAnimation: (component: ComponentType) => QueuedAnimation | null;
  isNodeAnimating: (nodeId: string) => boolean;
  clearQueues: () => void;
}

const MAX_CONCURRENT_ANIMATIONS = 10;

export const useAnimationQueueStore = create<AnimationQueueState>((set, get) => ({
  graphQueue: [],
  priorityQueueQueue: [],
  workerGridQueue: [],
  graphAnimating: new Map(),
  priorityQueueAnimating: new Map(),
  workerGridAnimating: new Map(),
  nodeAnimationStatus: new Map(),

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

  startAnimation: (animationId, component) => {
    const animation = get().getNextReadyAnimation(component);
    if (!animation) return;

    const animationState: AnimationState = {
      status: 'animating',
      startTime: Date.now(),
    };

    set((state) => {
      const newNodeAnimationStatus = new Map(state.nodeAnimationStatus);
      newNodeAnimationStatus.set(animation.nodeId, { component, animationId });

      switch (component) {
        case 'graph': {
          const newAnimating = new Map(state.graphAnimating);
          newAnimating.set(animationId, animationState);
          return {
            graphAnimating: newAnimating,
            graphQueue: state.graphQueue.filter((a) => a.id !== animationId),
            nodeAnimationStatus: newNodeAnimationStatus,
          };
        }
        case 'priority_queue': {
          const newAnimating = new Map(state.priorityQueueAnimating);
          newAnimating.set(animationId, animationState);
          return {
            priorityQueueAnimating: newAnimating,
            priorityQueueQueue: state.priorityQueueQueue.filter((a) => a.id !== animationId),
            nodeAnimationStatus: newNodeAnimationStatus,
          };
        }
        case 'worker_grid': {
          const newAnimating = new Map(state.workerGridAnimating);
          newAnimating.set(animationId, animationState);
          return {
            workerGridAnimating: newAnimating,
            workerGridQueue: state.workerGridQueue.filter((a) => a.id !== animationId),
            nodeAnimationStatus: newNodeAnimationStatus,
          };
        }
        default:
          return state;
      }
    });
  },

  completeAnimation: (animationId, component) => {
    set((state) => {
      let nodeIdToRemove: string | null = null;

      // Find the nodeId associated with this animation
      switch (component) {
        case 'graph': {
          const animationState = state.graphAnimating.get(animationId);
          if (animationState) {
            // Find nodeId from nodeAnimationStatus
            Array.from(state.nodeAnimationStatus.entries()).forEach(([nodeId, status]) => {
              if (status.animationId === animationId) {
                nodeIdToRemove = nodeId;
              }
            });
          }
          break;
        }
        case 'priority_queue': {
          const animationState = state.priorityQueueAnimating.get(animationId);
          if (animationState) {
            Array.from(state.nodeAnimationStatus.entries()).forEach(([nodeId, status]) => {
              if (status.animationId === animationId) {
                nodeIdToRemove = nodeId;
              }
            });
          }
          break;
        }
        case 'worker_grid': {
          const animationState = state.workerGridAnimating.get(animationId);
          if (animationState) {
            Array.from(state.nodeAnimationStatus.entries()).forEach(([nodeId, status]) => {
              if (status.animationId === animationId) {
                nodeIdToRemove = nodeId;
              }
            });
          }
          break;
        }
      }

      const newNodeAnimationStatus = new Map(state.nodeAnimationStatus);
      if (nodeIdToRemove) {
        newNodeAnimationStatus.delete(nodeIdToRemove);
      }

      switch (component) {
        case 'graph': {
          const newAnimating = new Map(state.graphAnimating);
          newAnimating.delete(animationId);
          return {
            graphAnimating: newAnimating,
            nodeAnimationStatus: newNodeAnimationStatus,
          };
        }
        case 'priority_queue': {
          const newAnimating = new Map(state.priorityQueueAnimating);
          newAnimating.delete(animationId);
          return {
            priorityQueueAnimating: newAnimating,
            nodeAnimationStatus: newNodeAnimationStatus,
          };
        }
        case 'worker_grid': {
          const newAnimating = new Map(state.workerGridAnimating);
          newAnimating.delete(animationId);
          return {
            workerGridAnimating: newAnimating,
            nodeAnimationStatus: newNodeAnimationStatus,
          };
        }
        default:
          return state;
      }
    });
  },

  getNextReadyAnimation: (component) => {
    const state = get();
    let queue: QueuedAnimation[];
    let animatingMap: Map<string, AnimationState>;

    switch (component) {
      case 'graph':
        queue = state.graphQueue;
        animatingMap = state.graphAnimating;
        break;
      case 'priority_queue':
        queue = state.priorityQueueQueue;
        animatingMap = state.priorityQueueAnimating;
        break;
      case 'worker_grid':
        queue = state.workerGridQueue;
        animatingMap = state.workerGridAnimating;
        break;
      default:
        return null;
    }

    // Check if we've hit the concurrent animation limit
    if (animatingMap.size >= MAX_CONCURRENT_ANIMATIONS) {
      return null;
    }

    // Find the first animation in the queue whose node is not currently animating
    for (const animation of queue) {
      if (!state.isNodeAnimating(animation.nodeId)) {
        return animation;
      }
    }

    return null;
  },

  isNodeAnimating: (nodeId) => {
    return get().nodeAnimationStatus.has(nodeId);
  },

  clearQueues: () => {
    set({
      graphQueue: [],
      priorityQueueQueue: [],
      workerGridQueue: [],
      graphAnimating: new Map(),
      priorityQueueAnimating: new Map(),
      workerGridAnimating: new Map(),
      nodeAnimationStatus: new Map(),
    });
  },
}));
