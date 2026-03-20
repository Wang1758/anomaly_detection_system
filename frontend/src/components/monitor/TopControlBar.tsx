import { useState, useEffect, useCallback } from 'react';
import { Video, FolderOpen, Play, Square } from 'lucide-react';
import { CrystalButton } from '../ui/CrystalButton';
import { useAppStore } from '../../stores/appStore';

const DEFAULT_LOCAL_VIDEO = '../data/videos/1.mp4';

export function TopControlBar() {
  const { config, setConfig, pipelineRunning, setPipelineRunning } = useAppStore();
  const [sourceType, setSourceType] = useState<'rtsp' | 'local'>(config?.source_type || 'local');
  const [sourceAddr, setSourceAddr] = useState(config?.source_addr || DEFAULT_LOCAL_VIDEO);
  const [fps, setFps] = useState(config?.fps || 30);

  const syncPipelineStatus = useCallback(async () => {
    try {
      const res = await fetch('/api/pipeline/status');
      if (!res.ok) return;
      const data = await res.json();
      setPipelineRunning(Boolean(data?.running));
    } catch {
      // keep current UI state on transient network errors
    }
  }, [setPipelineRunning]);

  useEffect(() => {
    if (config) {
      setSourceType(config.source_type);
      setSourceAddr(config.source_addr || (config.source_type === 'local' ? DEFAULT_LOCAL_VIDEO : ''));
      setFps(config.fps);
    }
  }, [config]);

  useEffect(() => {
    syncPipelineStatus();
    const timer = window.setInterval(syncPipelineStatus, 2000);
    return () => window.clearInterval(timer);
  }, [syncPipelineStatus]);

  const handleApply = async () => {
    try {
      await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ source_type: sourceType, source_addr: sourceAddr, fps }),
      });
      setConfig({ ...config!, source_type: sourceType, source_addr: sourceAddr, fps });

      await fetch('/api/pipeline/start', { method: 'POST' });
      await syncPipelineStatus();
    } catch (e) {
      console.error('Apply failed:', e);
    }
  };

  const handleFpsChange = async (nextFps: number) => {
    if (nextFps === fps) return;

    setFps(nextFps);

    try {
      await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fps: nextFps }),
      });

      if (config) {
        setConfig({ ...config, fps: nextFps });
      }
    } catch (e) {
      console.error('FPS apply failed:', e);
    }
  };

  const handleStop = async () => {
    try {
      await fetch('/api/pipeline/stop', { method: 'POST' });
      await syncPipelineStatus();
    } catch (e) {
      console.error('Stop failed:', e);
    }
  };

  return (
    <div className="glass rounded-2xl px-5 py-3.5 flex items-center gap-4.5 glow-border">
      {/* Source type toggle */}
      <div className="flex rounded-xl overflow-hidden border border-white/40">
        <button
          onClick={() => setSourceType('rtsp')}
          className={`flex items-center gap-2 px-3.5 py-2 text-sm font-medium transition-all
            ${sourceType === 'rtsp'
              ? 'bg-blue-500/15 text-blue-600'
              : 'text-gray-500 hover:bg-white/40'
            }`}
        >
          <Video size={16} /> RTSP
        </button>
        <button
          onClick={() => setSourceType('local')}
          className={`flex items-center gap-2 px-3.5 py-2 text-sm font-medium transition-all
            ${sourceType === 'local'
              ? 'bg-blue-500/15 text-blue-600'
              : 'text-gray-500 hover:bg-white/40'
            }`}
        >
          <FolderOpen size={16} /> 本地
        </button>
      </div>

      {/* Source address input */}
      <div className="flex-1">
        <input
          type="text"
          value={sourceAddr}
          onChange={(e) => setSourceAddr(e.target.value)}
          placeholder={sourceType === 'rtsp' ? 'rtsp://192.168.1.100:554/stream' : '/path/to/video/or/images/'}
          className="w-full bg-white/30 border border-white/40 rounded-xl px-3.5 py-2
            text-base text-gray-700 placeholder-gray-400 outline-none focus:border-blue-400/50
            transition-colors"
        />
      </div>

      {/* FPS toggle */}
      <div className="flex rounded-xl overflow-hidden border border-white/40">
        <button
          onClick={() => handleFpsChange(30)}
          className={`px-3.5 py-2 text-sm font-medium transition-all
            ${fps === 30 ? 'bg-blue-500/15 text-blue-600' : 'text-gray-500 hover:bg-white/40'}`}
        >
          30 FPS
        </button>
        <button
          onClick={() => handleFpsChange(60)}
          className={`px-3.5 py-2 text-sm font-medium transition-all
            ${fps === 60 ? 'bg-blue-500/15 text-blue-600' : 'text-gray-500 hover:bg-white/40'}`}
        >
          60 FPS
        </button>
      </div>

      {/* Action buttons */}
      <CrystalButton variant="primary" size="sm" onClick={handleApply}>
        <span className="flex items-center gap-2">
          <Play size={16} /> 启动
        </span>
      </CrystalButton>

      {pipelineRunning && (
        <CrystalButton variant="danger" size="sm" onClick={handleStop}>
          <span className="flex items-center gap-2">
            <Square size={16} /> 停止
          </span>
        </CrystalButton>
      )}
    </div>
  );
}
