import { useEffect, useRef } from 'react';
import type { DetectionFrame } from '../types';

type DetectionListener = (frame: DetectionFrame) => void;

const listeners = new Set<DetectionListener>();

export function emitDetection(frame: DetectionFrame) {
  listeners.forEach((fn) => fn(frame));
}

/**
 * Subscribe to real-time detection overlay data.
 * The listener is called outside of React's render cycle
 * to avoid unnecessary re-renders at high frame rates.
 */
export function useDetectionStream(listener: DetectionListener) {
  const listenerRef = useRef(listener);
  listenerRef.current = listener;

  useEffect(() => {
    const wrapped: DetectionListener = (frame) => listenerRef.current(frame);
    listeners.add(wrapped);
    return () => {
      listeners.delete(wrapped);
    };
  }, []);
}
