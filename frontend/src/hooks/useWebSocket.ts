import { useEffect, useRef, useCallback } from 'react';
import type { AlertEvent, DetectionFrame } from '../types';
import { emitDetection } from './useDetectionStream';

const BUFFER_DELAY_MS = 500;
const DRAIN_INTERVAL_MS = 120;
const MAX_BUFFER_SIZE = 1000;

export function useWebSocket(onAlert: (event: AlertEvent) => void) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const bufferRef = useRef<AlertEvent[]>([]);
  const warmupTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const drainTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const closingRef = useRef(false);

  const stopDrain = useCallback(() => {
    if (warmupTimerRef.current) {
      clearTimeout(warmupTimerRef.current);
      warmupTimerRef.current = null;
    }
    if (drainTimerRef.current) {
      clearInterval(drainTimerRef.current);
      drainTimerRef.current = null;
    }
  }, []);

  const ensureDrain = useCallback(() => {
    if (warmupTimerRef.current || drainTimerRef.current) {
      return;
    }

    warmupTimerRef.current = setTimeout(() => {
      warmupTimerRef.current = null;

      if (drainTimerRef.current) {
        return;
      }

      drainTimerRef.current = setInterval(() => {
        const next = bufferRef.current.shift();
        if (next) {
          onAlert(next);
          return;
        }

        if (drainTimerRef.current) {
          clearInterval(drainTimerRef.current);
          drainTimerRef.current = null;
        }
      }, DRAIN_INTERVAL_MS);
    }, BUFFER_DELAY_MS);
  }, [onAlert]);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/ws/events`;

    const ws = new WebSocket(url);
    wsRef.current = ws;
    closingRef.current = false;

    ws.onopen = () => {
      console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);

        if (data.type === 'detections') {
          emitDetection(data as DetectionFrame);
          return;
        }

        if (data.type === 'alert') {
          if (bufferRef.current.length >= MAX_BUFFER_SIZE) {
            bufferRef.current.shift();
          }
          bufferRef.current.push(data as AlertEvent);
          ensureDrain();
        }
      } catch (e) {
        console.error('WebSocket parse error:', e);
      }
    };

    ws.onclose = () => {
      if (closingRef.current) {
        return;
      }
      console.log('WebSocket disconnected, reconnecting in 3s...');
      reconnectTimer.current = setTimeout(connect, 3000);
    };

    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
      ws.close();
    };
  }, [onAlert]);

  useEffect(() => {
    connect();
    return () => {
      closingRef.current = true;
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
      stopDrain();
      bufferRef.current = [];
      wsRef.current?.close();
    };
  }, [connect, stopDrain]);

  return wsRef;
}
