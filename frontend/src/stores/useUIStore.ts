import { create } from 'zustand';

interface UIState {
  isPreparing: boolean;
  isGraphPrepared: boolean;
  isProcessing: boolean;
  setIsPreparing: (isPreparing: boolean) => void;
  setIsGraphPrepared: (isGraphPrepared: boolean) => void;
  setIsProcessing: (isProcessing: boolean) => void;
  resetUI: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  isPreparing: false,
  isGraphPrepared: false,
  isProcessing: false,

  setIsPreparing: (isPreparing) => set({ isPreparing }),

  setIsGraphPrepared: (isGraphPrepared) => set({ isGraphPrepared }),

  setIsProcessing: (isProcessing) => set({ isProcessing }),

  resetUI: () =>
    set({
      isPreparing: false,
      isGraphPrepared: false,
      isProcessing: false,
    }),
}));
