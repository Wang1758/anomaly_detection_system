import { useState, useEffect } from 'react';
import { Video, FolderOpen, Play, Square } from 'lucide-react';
import { CrystalButton } from '../ui/CrystalButton';
import { useAppStore } from '../../stores/appStore';

export function TopControlBar() {
  const { config, setConfig, pipelineRunning, setPipelineRunning } = useAppStore();
  const [sourceType, setSourceType] = useState<'rtsp' | 'local'>(config?.source_type || 'local');
  const [sourceAddr, setSourceAddr] = useState(config?.source_addr || '');
  const [fps, setFps] = useState(config?.fps || 30);

  useEffect(() => {
    if (config) {
      setSourceType(config.source_type);
      setSourceAddr(config.source_addr);
      setFps(config.fps);
    }
  }, [config]);

  const handleApply = async () => {
    try {
      await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ source_type: sourceType, source_addr: sourceAddr, fps }),
      });
      setConfig({ ...config!, source_type: sourceType, source_addr: sourceAddr, fps });

      if (!pipelineRunning) {
        await fetch('/api/pipeline/start', { method: 'POST' });
        setPipelineRunning(true);
      }
    } catch (e) {
      console.error('Apply failed:', e);
    }
  };

  const handleStop = async () => {
    try {
      await fetch('/api/pipeline/stop', { method: 'POST' });
      setPipelineRunning(false);
    } catch (e) {
      console.error('Stop failed:', e);
    }
  };

  return (
    <div className="glass rounded-2xl px-5 py-3 flex items-center gap-4 glow-border">
      {/* Source type toggle */}
      <div className="flex rounded-xl overflow-hidden border border-white/40">
        <button
          onClick={() => setSourceType('rtsp')}
          className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-all
            ${sourceType === 'rtsp'
              ? 'bg-blue-500/15 text-blue-600'
              : 'text-gray-500 hover:bg-white/40'
            }`}
        >
          <Video size={14} /> RTSP
        </button>
        <button
          onClick={() => setSourceType('local')}
          className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-all
            ${sourceType === 'local'
              ? 'bg-blue-500/15 text-blue-600'
              : 'text-gray-500 hover:bg-white/40'
            }`}
        >
          <FolderOpen size={14} /> 本地
        </button>
      </div>

      {/* Source address input */}
      <div className="flex-1">
        {sourceType === 'rtsp' ? (
          <input
            type="text"
            value={sourceAddr}
            onChange={(e) => setSourceAddr(e.target.value)}
            placeholder="rtsp://192.168.1.100:554/stream"
            className="w-full bg-white/30 border border-white/40 rounded-xl px-3 py-1.5
              text-sm text-gray-700 placeholder-gray-400 outline-none focus:border-blue-400/50
              transition-colors"
          />
        ) : (
          <div className="flex items-center gap-2">
            <FolderOpen size={14} className="text-gray-400" />
            <input
              type="text"
              value={sourceAddr}
              onChange={(e) => setSourceAddr(e.target.value)}
              placeholder="/path/to/video/or/images/"
              className="flex-1 bg-white/30 border border-white/40 rounded-xl px-3 py-1.5
                text-sm text-gray-700 placeholder-gray-400 outline-none focus:border-blue-400/50
                transition-colors font-mono text-xs"
            />
          </div>
        )}
      </div>

      {/* FPS toggle */}
      <div className="flex rounded-xl overflow-hidden border border-white/40">
        <button
          onClick={() => setFps(30)}
          className={`px-3 py-1.5 text-xs font-medium transition-all
            ${fps === 30 ? 'bg-blue-500/15 text-blue-600' : 'text-gray-500 hover:bg-white/40'}`}
        >
          30 FPS
        </button>
        <button
          onClick={() => setFps(60)}
          className={`px-3 py-1.5 text-xs font-medium transition-all
            ${fps === 60 ? 'bg-blue-500/15 text-blue-600' : 'text-gray-500 hover:bg-white/40'}`}
        >
          60 FPS
        </button>
      </div>

      {/* Action buttons */}
      <CrystalButton variant="primary" size="sm" onClick={handleApply}>
        <span className="flex items-center gap-1.5">
          <Play size={14} /> 应用
        </span>
      </CrystalButton>

      {pipelineRunning && (
        <CrystalButton variant="danger" size="sm" onClick={handleStop}>
          <span className="flex items-center gap-1.5">
            <Square size={14} /> 停止
          </span>
        </CrystalButton>
      )}
    </div>
  );
}
