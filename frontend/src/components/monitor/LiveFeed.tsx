import { useEffect, useRef, useState, useCallback } from 'react';
import { useAppStore } from '../../stores/appStore';
import type { DetectionMeta, LiveFrameMeta } from '../../types';

const NORMAL_COLOR = '#00ff00';
const UNCERTAIN_COLOR = '#ff3333';
const FONT = '14px "JetBrains Mono", "SF Mono", "Cascadia Code", monospace';

function drawBoxes(
  ctx: CanvasRenderingContext2D,
  detections: DetectionMeta[],
  frameWidth: number,
  frameHeight: number,
  canvasWidth: number,
  canvasHeight: number,
) {
  const cw = canvasWidth;
  const ch = canvasHeight;

  const imgRatio = frameWidth / frameHeight;
  const containerRatio = cw / ch;
  let renderW: number, renderH: number, offX: number, offY: number;

  if (imgRatio > containerRatio) {
    renderW = cw;
    renderH = cw / imgRatio;
    offX = 0;
    offY = (ch - renderH) / 2;
  } else {
    renderH = ch;
    renderW = ch * imgRatio;
    offX = (cw - renderW) / 2;
    offY = 0;
  }

  const sx = renderW / frameWidth;
  const sy = renderH / frameHeight;

  for (const det of detections) {
    const x1 = det.x1 * sx + offX;
    const y1 = det.y1 * sy + offY;
    const x2 = det.x2 * sx + offX;
    const y2 = det.y2 * sy + offY;
    const w = x2 - x1;
    const h = y2 - y1;

    const color = det.is_uncertain ? UNCERTAIN_COLOR : NORMAL_COLOR;

    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.strokeRect(x1, y1, w, h);

    const label = `${det.class_name} ${(det.confidence * 100).toFixed(0)}%${det.is_uncertain ? ' [?]' : ''}`;
    ctx.font = FONT;
    const tm = ctx.measureText(label);
    const textH = 18;
    const pad = 4;

    ctx.fillStyle = color;
    ctx.globalAlpha = 0.75;
    ctx.fillRect(x1, Math.max(0, y1 - textH - pad), tm.width + pad * 2, textH + pad);
    ctx.globalAlpha = 1.0;

    ctx.fillStyle = '#ffffff';
    ctx.fillText(label, x1 + pad, Math.max(textH, y1 - pad));
  }
}

export function LiveFeed() {
  const pipelineRunning = useAppStore((s) => s.pipelineRunning);
  const [actualFps, setActualFps] = useState(0);
  const [targetFps, setTargetFps] = useState(0);

  const canvasRef = useRef<HTMLCanvasElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const processingRef = useRef(false);
  const pendingBufferRef = useRef<ArrayBuffer | null>(null);
  const latestMetaRef = useRef<LiveFrameMeta | null>(null);
  const latestBitmapRef = useRef<ImageBitmap | null>(null);
  const decoderRef = useRef(new TextDecoder());

  const drawLiveFrame = useCallback((meta: LiveFrameMeta, bitmap: ImageBitmap) => {
    const canvas = canvasRef.current;
    if (!canvas || !canvas.clientWidth || !canvas.clientHeight) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const cw = canvas.clientWidth;
    const ch = canvas.clientHeight;
    if (canvas.width !== cw || canvas.height !== ch) {
      canvas.width = cw;
      canvas.height = ch;
    }

    ctx.clearRect(0, 0, cw, ch);

    const imgRatio = meta.frame_width / meta.frame_height;
    const containerRatio = cw / ch;
    let renderW: number, renderH: number, offX: number, offY: number;

    if (imgRatio > containerRatio) {
      renderW = cw;
      renderH = cw / imgRatio;
      offX = 0;
      offY = (ch - renderH) / 2;
    } else {
      renderH = ch;
      renderW = ch * imgRatio;
      offX = (cw - renderW) / 2;
      offY = 0;
    }

    ctx.drawImage(bitmap, offX, offY, renderW, renderH);
    drawBoxes(ctx, meta.detections, meta.frame_width, meta.frame_height, cw, ch);
  }, []);

  const decodeAndRender = useCallback(async (buffer: ArrayBuffer) => {
    if (buffer.byteLength < 5) return;

    const view = new DataView(buffer);
    const metaLen = view.getUint32(0, false);
    if (metaLen <= 0 || 4 + metaLen >= buffer.byteLength) return;

    const metaBytes = new Uint8Array(buffer, 4, metaLen);
    const jpegBytes = new Uint8Array(buffer, 4 + metaLen);

    const meta = JSON.parse(decoderRef.current.decode(metaBytes)) as LiveFrameMeta;
    if (meta.type !== 'live_frame') return;

    const bitmap = await createImageBitmap(new Blob([jpegBytes], { type: 'image/jpeg' }));

    latestMetaRef.current = meta;
    const prevBitmap = latestBitmapRef.current;
    latestBitmapRef.current = bitmap;
    if (prevBitmap) {
      prevBitmap.close();
    }

    drawLiveFrame(meta, bitmap);
  }, [drawLiveFrame]);

  const pumpLatestBuffer = useCallback(async () => {
    if (processingRef.current) return;
    processingRef.current = true;

    try {
      while (pendingBufferRef.current) {
        const buffer = pendingBufferRef.current;
        pendingBufferRef.current = null;
        await decodeAndRender(buffer);
      }
    } finally {
      processingRef.current = false;
    }
  }, [decodeAndRender]);

  useEffect(() => {
    if (!pipelineRunning) {
      pendingBufferRef.current = null;
      latestMetaRef.current = null;
      if (latestBitmapRef.current) {
        latestBitmapRef.current.close();
        latestBitmapRef.current = null;
      }
      wsRef.current?.close();
      wsRef.current = null;

      const canvas = canvasRef.current;
      if (canvas) {
        const ctx = canvas.getContext('2d');
        ctx?.clearRect(0, 0, canvas.width, canvas.height);
      }
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/live`);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    ws.onmessage = (event) => {
      if (!(event.data instanceof ArrayBuffer)) return;
      pendingBufferRef.current = event.data;
      void pumpLatestBuffer();
    };

    ws.onerror = () => {
      ws.close();
    };

    return () => {
      ws.close();
      if (wsRef.current === ws) {
        wsRef.current = null;
      }
    };
  }, [pipelineRunning, pumpLatestBuffer]);

  // Redraw on resize
  useEffect(() => {
    const onResize = () => {
      const meta = latestMetaRef.current;
      const bitmap = latestBitmapRef.current;
      const canvas = canvasRef.current;
      if (!meta || !bitmap || !canvas || !canvas.clientWidth) return;
      drawLiveFrame(meta, bitmap);
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [drawLiveFrame]);

  useEffect(() => {
    return () => {
      if (latestBitmapRef.current) {
        latestBitmapRef.current.close();
      }
    };
  }, []);

  useEffect(() => {
    let active = true;

    const syncStatus = async () => {
      try {
        const res = await fetch('/api/pipeline/status');
        if (!res.ok || !active) return;
        const data = await res.json();
        if (!active) return;
        setActualFps(typeof data?.fps === 'number' ? data.fps : 0);
        setTargetFps(typeof data?.target_fps === 'number' ? data.target_fps : 0);
      } catch {
        if (!active) return;
        setActualFps(0);
      }
    };

    if (pipelineRunning) {
      void syncStatus();
      const timer = window.setInterval(syncStatus, 1000);
      return () => {
        active = false;
        window.clearInterval(timer);
      };
    }

    setActualFps(0);
    return () => {
      active = false;
    };
  }, [pipelineRunning]);

  return (
    <div className="glass rounded-2xl flex-1 flex items-center justify-center overflow-hidden glow-border relative">
      {pipelineRunning ? (
        <div className="w-full h-full relative">
          <div className="absolute top-3 right-3 z-20 bg-black/70 text-white text-xs px-3 py-1.5 rounded-lg font-mono pointer-events-none border border-white/15 shadow-sm">
            实时 {Number.isFinite(actualFps) ? actualFps.toFixed(1) : '--'} FPS{targetFps > 0 ? ` / 目标 ${targetFps}` : ''}
          </div>
          <canvas
            ref={canvasRef}
            className="absolute inset-0 w-full h-full"
            style={{ zIndex: 10, backgroundColor: 'transparent' }}
          />
        </div>
      ) : (
        <div className="text-center text-gray-400">
          <div className="w-24 h-24 rounded-full bg-white/30 flex items-center justify-center mx-auto mb-4">
            <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
              <polygon points="5 3 19 12 5 21 5 3" />
            </svg>
          </div>
          <p className="text-base font-medium">等待视频流</p>
          <p className="text-sm mt-1 text-gray-400/70">请在顶部配置视频源并点击"应用"</p>
        </div>
      )}
    </div>
  );
}
