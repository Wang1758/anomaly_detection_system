import { create } from 'zustand';
import type { AlertEvent, Sample, SystemConfig, TrainingRun, ViewType } from '../types';

interface AppState {
  currentView: ViewType;
  setCurrentView: (view: ViewType) => void;

  pendingAlerts: AlertEvent[];
  setPendingAlerts: (alerts: AlertEvent[]) => void;
  addAlert: (alert: AlertEvent) => void;
  removeAlert: (frameId: number) => void;
  clearAlerts: () => void;

  config: SystemConfig | null;
  setConfig: (config: SystemConfig) => void;

  samples: Sample[];
  setSamples: (samples: Sample[]) => void;

  trainingHistory: TrainingRun[];
  setTrainingHistory: (runs: TrainingRun[]) => void;

  pipelineRunning: boolean;
  setPipelineRunning: (running: boolean) => void;

  lightboxAlert: AlertEvent | null;
  setLightboxAlert: (alert: AlertEvent | null) => void;
}

export const useAppStore = create<AppState>((set) => ({
  currentView: 'monitor',
  setCurrentView: (view) => set({ currentView: view }),

  pendingAlerts: [],
  setPendingAlerts: (alerts) => {
    const deduped = alerts.reduce<AlertEvent[]>((acc, item) => {
      const idx = acc.findIndex((a) => a.frame_id === item.frame_id);
      if (idx >= 0) {
        acc[idx] = item;
      } else {
        acc.push(item);
      }
      return acc;
    }, []);
    set({ pendingAlerts: deduped.slice(0, 50) });
  },
  addAlert: (alert) =>
    set((state) => {
      const existed = state.pendingAlerts.some((a) => a.frame_id === alert.frame_id);
      if (existed) {
        return {
          pendingAlerts: state.pendingAlerts
            .map((a) => (a.frame_id === alert.frame_id ? alert : a))
            .slice(0, 50),
        };
      }
      return { pendingAlerts: [alert, ...state.pendingAlerts].slice(0, 50) };
    }),
  removeAlert: (frameId) =>
    set((state) => ({
      pendingAlerts: state.pendingAlerts.filter((a) => a.frame_id !== frameId),
    })),
  clearAlerts: () => set({ pendingAlerts: [] }),

  config: null,
  setConfig: (config) => set({ config }),

  samples: [],
  setSamples: (samples) => set({ samples }),

  trainingHistory: [],
  setTrainingHistory: (runs) => set({ trainingHistory: runs }),

  pipelineRunning: false,
  setPipelineRunning: (running) => set({ pipelineRunning: running }),

  lightboxAlert: null,
  setLightboxAlert: (alert) => set({ lightboxAlert: alert }),
}));
