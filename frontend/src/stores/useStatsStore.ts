import { create } from 'zustand';
import { Stats } from '../types/Stats';

const INITIAL_STATS: Stats = {
  GraphSize: 0,
  PriorityQueueSize: 0,
  WorkersPoolSize: 0,
  WorkersPoolBusy: 0,
  LaunchedCount: 0,
  CompletedCount: 0,
  FailedCount: 0,
};

interface StatsState {
  stats: Stats;
  setStats: (stats: Stats) => void;
  updateStats: (updates: Partial<Stats>) => void;
  resetStats: () => void;
}

export const useStatsStore = create<StatsState>((set) => ({
  stats: INITIAL_STATS,

  setStats: (stats) => set({ stats }),

  updateStats: (updates) =>
    set((state) => ({
      stats: { ...state.stats, ...updates },
    })),

  resetStats: () => set({ stats: INITIAL_STATS }),
}));
