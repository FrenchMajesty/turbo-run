/**
 * Zustand store for choreography playback state
 */

import { create } from "zustand";
import { PlaybackSpeed, DEFAULT_PLAYBACK_SPEED } from "../constants/timing";

export enum PlaybackState {
  IDLE = "IDLE",
  LOADED = "LOADED",
  RUNNING = "RUNNING",
  PAUSED = "PAUSED",
  COMPLETED = "COMPLETED",
}

interface ChoreographyStore {
  // Playback state
  state: PlaybackState;
  speed: PlaybackSpeed;
  scenarioName: string | null;

  // Progress tracking
  totalNodes: number;
  completedNodes: number;
  failedNodes: number;

  // Actions
  setState: (state: PlaybackState) => void;
  setSpeed: (speed: PlaybackSpeed) => void;
  setScenario: (name: string, totalNodes: number) => void;
  incrementCompleted: () => void;
  incrementFailed: () => void;
  reset: () => void;
}

const initialState = {
  state: PlaybackState.IDLE,
  speed: DEFAULT_PLAYBACK_SPEED as PlaybackSpeed,
  scenarioName: null,
  totalNodes: 0,
  completedNodes: 0,
  failedNodes: 0,
};

export const useChoreographyStore = create<ChoreographyStore>((set) => ({
  ...initialState,

  setState: (state) => set({ state }),

  setSpeed: (speed) => set({ speed }),

  setScenario: (name, totalNodes) =>
    set({
      scenarioName: name,
      totalNodes,
      completedNodes: 0,
      failedNodes: 0,
      state: PlaybackState.LOADED,
    }),

  incrementCompleted: () =>
    set((state) => ({
      completedNodes: state.completedNodes + 1,
    })),

  incrementFailed: () =>
    set((state) => ({
      failedNodes: state.failedNodes + 1,
    })),

  reset: () => set(initialState),
}));
