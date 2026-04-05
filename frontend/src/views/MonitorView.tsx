import { useEffect, useRef, useState, useCallback } from 'react';
import { TopControlBar } from '../components/monitor/TopControlBar';
import { LiveFeed } from '../components/monitor/LiveFeed';
import { PendingQueue } from '../components/monitor/PendingQueue';
import { Lightbox } from '../components/ui/Lightbox';
import { CrystalButton } from '../components/ui/CrystalButton';
import { useAppStore } from '../stores/appStore';
import { Check, X, ZoomIn, ZoomOut, RotateCcw } from 'lucide-react';
import type { DetectionMeta } from '../types';

const NORMAL_COLOR = '#00ff00';
const UNCERTAIN_COLOR = '#ff3333';
const FONT = '14px "JetBrains Mono", "SF Mono", monospace';

function drawLightboxBoxes(
  canvas: HTMLCanvasElement,
  img: HTMLImageElement,
  detections: DetectionMeta[],
) {
  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  const cw = img.clientWidth;
  const ch = img.clientHeight;
  canvas.width = cw;
  canvas.height = ch;

  const natW = img.naturalWidth;
  const natH = img.naturalHeight;
  if (!natW || !natH) return;

  const imgRatio = natW / natH;
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

  const sx = renderW / natW;
  const sy = renderH / natH;

  ctx.clearRect(0, 0, cw, ch);

  for (const det of detections) {
    const x1 = det.x1 * sx + offX;
    const y1 = det.y1 * sy + offY;
    const w = (det.x2 - det.x1) * sx;
    const h = (det.y2 - det.y1) * sy;
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

export function MonitorView() {
  const { lightboxAlert, setLightboxAlert, removeAlert } = useAppStore();
  const [zoom, setZoom] = useState(1);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const dragStartRef = useRef({ x: 0, y: 0 });
  const lightboxImgRef = useRef<HTMLImageElement>(null);
  const lightboxCanvasRef = useRef<HTMLCanvasElement>(null);

  const redrawLightboxBoxes = useCallback(() => {
    const img = lightboxImgRef.current;
    const canvas = lightboxCanvasRef.current;
    if (!img || !canvas || !lightboxAlert) return;
    drawLightboxBoxes(canvas, img, lightboxAlert.detections);
  }, [lightboxAlert]);

  useEffect(() => {
    if (lightboxAlert) {
      setZoom(1);
      setOffset({ x: 0, y: 0 });
      setDragging(false);
    }
  }, [lightboxAlert]);

  const clampZoom = (value: number) => Math.min(6, Math.max(1, value));

  const zoomBy = (delta: number) => {
    setZoom((z) => clampZoom(z + delta));
  };

  const resetView = () => {
    setZoom(1);
    setOffset({ x: 0, y: 0 });
  };

  const handleLightboxLabel = async (label: boolean) => {
    if (!lightboxAlert) return;
    try {
      const endpoint = lightboxAlert.sample_id
        ? `/api/samples/${lightboxAlert.sample_id}/label`
        : `/api/samples/frame/${lightboxAlert.frame_id}/label`;
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        console.error('Label failed:', data?.error || 'unknown error');
        return;
      }
      removeAlert(lightboxAlert.frame_id);
      setLightboxAlert(null);
    } catch (e) {
      console.error('Label failed:', e);
    }
  };

  return (
    <div className="h-full flex flex-col gap-4">
      <TopControlBar />

      <div className="flex-1 flex gap-4 min-h-0">
        <LiveFeed />
        <PendingQueue />
      </div>

      {/* Lightbox for expanded alert view */}
      <Lightbox open={!!lightboxAlert} onClose={() => setLightboxAlert(null)}>
        {lightboxAlert && (
          <div>
            <div className="mb-4 flex items-center justify-between gap-2">
              <span className="text-sm text-gray-500">滚轮缩放，拖拽平移，双击重置</span>
              <div className="flex items-center gap-2">
                <CrystalButton variant="primary" size="sm" onClick={() => zoomBy(-0.25)}>
                  <ZoomOut size={16} />
                </CrystalButton>
                <span className="text-sm font-mono text-gray-600 min-w-14 text-center">{Math.round(zoom * 100)}%</span>
                <CrystalButton variant="primary" size="sm" onClick={() => zoomBy(0.25)}>
                  <ZoomIn size={16} />
                </CrystalButton>
                <CrystalButton variant="primary" size="sm" onClick={resetView}>
                  <RotateCcw size={16} />
                </CrystalButton>
              </div>
            </div>

            <div
              className="w-full h-[70vh] rounded-xl mb-4 bg-black/10 overflow-hidden cursor-grab active:cursor-grabbing"
              onWheel={(e) => {
                e.preventDefault();
                const step = e.deltaY > 0 ? -0.15 : 0.15;
                setZoom((z) => clampZoom(z + step));
              }}
              onMouseDown={(e) => {
                setDragging(true);
                dragStartRef.current = { x: e.clientX - offset.x, y: e.clientY - offset.y };
              }}
              onMouseMove={(e) => {
                if (!dragging || zoom <= 1) return;
                setOffset({
                  x: e.clientX - dragStartRef.current.x,
                  y: e.clientY - dragStartRef.current.y,
                });
              }}
              onMouseUp={() => setDragging(false)}
              onMouseLeave={() => setDragging(false)}
              onDoubleClick={resetView}
            >
              <div className="w-full h-full relative">
                <img
                  ref={lightboxImgRef}
                  src={lightboxAlert.image_url}
                  alt={`Frame ${lightboxAlert.frame_id}`}
                  className="w-full h-full object-contain select-none"
                  draggable={false}
                  onLoad={redrawLightboxBoxes}
                  style={{
                    transform: `translate(${offset.x}px, ${offset.y}px) scale(${zoom})`,
                    transformOrigin: 'center center',
                  }}
                />
                <canvas
                  ref={lightboxCanvasRef}
                  className="absolute inset-0 w-full h-full pointer-events-none"
                  style={{
                    transform: `translate(${offset.x}px, ${offset.y}px) scale(${zoom})`,
                    transformOrigin: 'center center',
                  }}
                />
              </div>
            </div>
            <div className="flex items-center justify-between mb-4">
              <div>
                <span className="text-base font-semibold text-gray-700">
                  Frame #{lightboxAlert.frame_id}
                </span>
                <p className="text-sm text-gray-400 mt-1">
                  {lightboxAlert.detections.filter((d) => d.is_uncertain).length} 个不确定目标 |{' '}
                  {lightboxAlert.timestamp}
                </p>
              </div>
            </div>

            <div className="flex gap-3 justify-center">
              <CrystalButton
                variant="success"
                size="lg"
                onClick={() => handleLightboxLabel(true)}
              >
                <span className="flex items-center gap-2">
                  <Check size={18} /> 确认 (正样本)
                </span>
              </CrystalButton>
              <CrystalButton
                variant="danger"
                size="lg"
                onClick={() => handleLightboxLabel(false)}
              >
                <span className="flex items-center gap-2">
                  <X size={18} /> 误报 (负样本)
                </span>
              </CrystalButton>
            </div>
          </div>
        )}
      </Lightbox>
    </div>
  );
}
