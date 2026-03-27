import { useEffect, useRef, useState, useCallback } from 'react';
import { useAppStore } from '../../stores/appStore';
import { useDetectionStream } from '../../hooks/useDetectionStream';
import type { DetectionFrame, DetectionMeta } from '../../types';

const NORMAL_COLOR = '#00ff00';
const UNCERTAIN_COLOR = '#ff3333';
const FONT = '14px "JetBrains Mono", "SF Mono", "Cascadia Code", monospace';

function drawBoxes(
  ctx: CanvasRenderingContext2D,
  detections: DetectionMeta[],
  frameWidth: number,
  frameHeight: number,
  img: HTMLImageElement,
) {
  const cw = img.clientWidth;
  const ch = img.clientHeight;
  ctx.canvas.width = cw;
  ctx.canvas.height = ch;

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

  ctx.clearRect(0, 0, cw, ch);

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

  const imgRef = useRef<HTMLImageElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const latestFrameRef = useRef<DetectionFrame | null>(null);

  const handleDetection = useCallback((frame: DetectionFrame) => {
    latestFrameRef.current = frame;
    const canvas = canvasRef.current;
    const img = imgRef.current;
    if (!canvas || !img || !img.clientWidth) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    drawBoxes(ctx, frame.detections, frame.frame_width, frame.frame_height, img);
  }, []);

  useDetectionStream(handleDetection);

  useEffect(() => {
    if (!pipelineRunning) {
      latestFrameRef.current = null;
      const canvas = canvasRef.current;
      if (canvas) {
        const ctx = canvas.getContext('2d');
        ctx?.clearRect(0, 0, canvas.width, canvas.height);
      }
    }
  }, [pipelineRunning]);

  // Redraw on resize
  useEffect(() => {
    const onResize = () => {
      const frame = latestFrameRef.current;
      const canvas = canvasRef.current;
      const img = imgRef.current;
      if (!frame || !canvas || !img || !img.clientWidth) return;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      drawBoxes(ctx, frame.detections, frame.frame_width, frame.frame_height, img);
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
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
          <img
            ref={imgRef}
            src="/api/stream/mjpeg"
            alt="Live Feed"
            className="w-full h-full object-contain"
          />
          <canvas
            ref={canvasRef}
            className="absolute inset-0 w-full h-full pointer-events-none"
            style={{ zIndex: 10 }}
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
