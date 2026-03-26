import { useEffect, useState } from 'react';
import { useAppStore } from '../../stores/appStore';

export function LiveFeed() {
  const pipelineRunning = useAppStore((s) => s.pipelineRunning);
  const [actualFps, setActualFps] = useState(0);
  const [targetFps, setTargetFps] = useState(0);

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
            src="/api/stream/mjpeg"
            alt="Live Feed"
            className="w-full h-full object-contain"
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
