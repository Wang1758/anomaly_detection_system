import { motion, AnimatePresence } from 'framer-motion';
import { AlertTriangle, Check, X, Bot } from 'lucide-react';
import { CrystalButton } from '../ui/CrystalButton';
import { useAppStore } from '../../stores/appStore';

export function PendingQueue() {
  const { pendingAlerts, removeAlert, setLightboxAlert } = useAppStore();

  const handleLabel = async (frameId: number, label: boolean) => {
    try {
      // Find the sample ID from the backend
      const res = await fetch(`/api/samples?status=pending`);
      const samples = await res.json();
      const sample = samples.find((s: { frame_id: number }) => s.frame_id === frameId);
      if (sample) {
        await fetch(`/api/samples/${sample.id}/label`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ label }),
        });
      }
      removeAlert(frameId);
    } catch (e) {
      console.error('Label failed:', e);
    }
  };

  const handleAIJudge = async () => {
    try {
      await fetch('/api/samples/ai-judge', { method: 'POST' });
      useAppStore.getState().clearAlerts();
    } catch (e) {
      console.error('AI judge failed:', e);
    }
  };

  return (
    <div className="w-72 glass rounded-2xl flex flex-col glow-border">
      {/* Header */}
      <div className="px-4 py-3 border-b border-white/20 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <AlertTriangle size={16} className="text-amber-500" />
          <span className="text-sm font-semibold text-gray-700">待处理队列</span>
        </div>
        <span className="text-xs text-gray-400 bg-white/40 px-2 py-0.5 rounded-full">
          {pendingAlerts.length}
        </span>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        <AnimatePresence mode="popLayout">
          {pendingAlerts.map((alert) => (
            <motion.div
              key={alert.frame_id}
              layout
              initial={{ opacity: 0, x: 50, scale: 0.9 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: -50, scale: 0.9 }}
              transition={{ type: 'spring', damping: 20 }}
              className="bg-white/50 rounded-xl p-2 border border-white/40
                hover:bg-white/70 transition-colors group"
            >
              <div
                className="relative cursor-pointer overflow-hidden rounded-lg mb-2"
                onClick={() => setLightboxAlert(alert)}
              >
                <img
                  src={alert.image_url}
                  alt={`Frame ${alert.frame_id}`}
                  className="w-full h-28 object-cover"
                />
                <div className="absolute top-1 right-1 bg-red-500/80 text-white text-[10px]
                  px-1.5 py-0.5 rounded-md font-medium">
                  {alert.detections.filter((d) => d.is_uncertain).length} 异常
                </div>
              </div>

              <div className="flex items-center justify-between">
                <span className="text-[10px] text-gray-400 font-mono">
                  #{alert.frame_id}
                </span>
                <div className="flex gap-1">
                  <CrystalButton
                    variant="success"
                    size="sm"
                    onClick={() => handleLabel(alert.frame_id, true)}
                  >
                    <Check size={12} />
                  </CrystalButton>
                  <CrystalButton
                    variant="danger"
                    size="sm"
                    onClick={() => handleLabel(alert.frame_id, false)}
                  >
                    <X size={12} />
                  </CrystalButton>
                </div>
              </div>
            </motion.div>
          ))}
        </AnimatePresence>

        {pendingAlerts.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-gray-400">
            <Check size={24} className="mb-2 opacity-40" />
            <p className="text-xs">暂无待处理事件</p>
          </div>
        )}
      </div>

      {/* AI Judge Button */}
      {pendingAlerts.length > 0 && (
        <div className="p-3 border-t border-white/20">
          <CrystalButton
            variant="primary"
            size="sm"
            className="w-full"
            onClick={handleAIJudge}
          >
            <span className="flex items-center justify-center gap-1.5">
              <Bot size={14} /> AI 一键判断
            </span>
          </CrystalButton>
        </div>
      )}
    </div>
  );
}
