import { useAppStore } from '../../stores/appStore';

export function LiveFeed() {
  const pipelineRunning = useAppStore((s) => s.pipelineRunning);

  return (
    <div className="glass rounded-2xl flex-1 flex items-center justify-center overflow-hidden glow-border">
      {pipelineRunning ? (
        <img
          src="/api/stream/mjpeg"
          alt="Live Feed"
          className="w-full h-full object-contain"
        />
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
