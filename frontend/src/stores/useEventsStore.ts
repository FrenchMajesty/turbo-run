import { create } from 'zustand';
import { TurboEvent } from '../types/TurboEvent';

interface EventsState {
  events: TurboEvent[];
  addEvent: (event: TurboEvent) => void;
  clearEvents: () => void;
}

export const useEventsStore = create<EventsState>((set) => ({
  events: [],

  addEvent: (event) =>
    set((state) => {
      const newEvents = [event, ...state.events];
      // Keep only last 50 events
      return { events: newEvents.slice(0, 50) };
    }),

  clearEvents: () => set({ events: [] }),
}));
