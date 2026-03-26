import { useEffect, useRef, useState } from 'react';
import { TopControlBar } from '../components/monitor/TopControlBar';
import { LiveFeed } from '../components/monitor/LiveFeed';
import { PendingQueue } from '../components/monitor/PendingQueue';
import { Lightbox } from '../components/ui/Lightbox';
import { CrystalButton } from '../components/ui/CrystalButton';
import { useAppStore } from '../stores/appStore';
import { Check, X, ZoomIn, ZoomOut, RotateCcw } from 'lucide-react';

export function MonitorView() {
  const { lightboxAlert, setLightboxAlert, removeAlert } = useAppStore();
  const [zoom, setZoom] = useState(1);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const dragStartRef = useRef({ x: 0, y: 0 });

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
      const res = await fetch(`/api/samples/frame/${lightboxAlert.frame_id}/label`, {
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
              <img
                src={lightboxAlert.image_url}
                alt={`Frame ${lightboxAlert.frame_id}`}
                className="w-full h-full object-contain select-none"
                draggable={false}
                style={{
                  transform: `translate(${offset.x}px, ${offset.y}px) scale(${zoom})`,
                  transformOrigin: 'center center',
                }}
              />
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
