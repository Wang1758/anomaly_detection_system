import { useEffect, useRef, useCallback } from 'react';
import type { AlertEvent } from '../types';

export function useWebSocket(onAlert: (event: AlertEvent) => void) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/ws/events`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
      try {
        const data: AlertEvent = JSON.parse(event.data);
        if (data.type === 'alert') {
          onAlert(data);
        }
      } catch (e) {
        console.error('WebSocket parse error:', e);
      }
    };

    ws.onclose = () => {
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
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
      wsRef.current?.close();
    };
  }, [connect]);

  return wsRef;
}
