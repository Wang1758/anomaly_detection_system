import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { AlertTriangle, Check, X, Bot, Loader2 } from 'lucide-react';
import { CrystalButton } from '../ui/CrystalButton';
import { useAppStore } from '../../stores/appStore';

export function PendingQueue() {
  const { pendingAlerts, removeAlert, setLightboxAlert } = useAppStore();
  const [judging, setJudging] = useState(false);
  const [judgeStatus, setJudgeStatus] = useState<string | null>(null);

  const handleLabel = async (frameId: number, label: boolean) => {
    try {
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
    setJudging(true);
    setJudgeStatus(null);
    try {
      const res = await fetch('/api/samples/ai-judge', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) {
        setJudgeStatus(`研判失败: ${data.error || '未知错误'}`);
        return;
      }
      const method = data.method === 'llm' ? 'AI 大模型' : 'YOLO 重检测';
      const errors = (data.results || []).filter((r: { error?: string }) => r.error).length;
      const successCount = (data.count || 0) - errors;
      setJudgeStatus(`${method}完成: ${successCount} 成功${errors > 0 ? `, ${errors} 失败` : ''}`);
      useAppStore.getState().clearAlerts();
    } catch (e) {
      console.error('AI judge failed:', e);
      setJudgeStatus('网络错误，请重试');
    } finally {
      setJudging(false);
    }
  };

  return (
    <div className="w-80 glass rounded-2xl flex flex-col glow-border">
      {/* Header */}
      <div className="px-4 py-3.5 border-b border-white/20 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <AlertTriangle size={18} className="text-amber-500" />
          <span className="text-base font-semibold text-gray-700">待处理队列</span>
        </div>
        <span className="text-sm text-gray-500 bg-white/40 px-2.5 py-0.5 rounded-full">
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
                  className="w-full h-32 object-cover"
                />
                <div className="absolute top-1 right-1 bg-red-500/80 text-white text-xs
                  px-2 py-0.5 rounded-md font-medium">
                  {alert.detections.filter((d) => d.is_uncertain).length} 异常
                </div>
              </div>

              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-400 font-mono">
                  #{alert.frame_id}
                </span>
                <div className="flex gap-1">
                  <CrystalButton
                    variant="success"
                    size="sm"
                    onClick={() => handleLabel(alert.frame_id, true)}
                  >
                    <Check size={14} />
                  </CrystalButton>
                  <CrystalButton
                    variant="danger"
                    size="sm"
                    onClick={() => handleLabel(alert.frame_id, false)}
                  >
                    <X size={14} />
                  </CrystalButton>
                </div>
              </div>
            </motion.div>
          ))}
        </AnimatePresence>

        {pendingAlerts.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-gray-400">
            <Check size={28} className="mb-2 opacity-40" />
            <p className="text-sm">暂无待处理事件</p>
          </div>
        )}
      </div>

      {/* AI Judge Button */}
      {pendingAlerts.length > 0 && (
        <div className="p-3 border-t border-white/20 space-y-2">
          <CrystalButton
            variant="primary"
            size="sm"
            className="w-full"
            onClick={handleAIJudge}
            disabled={judging}
          >
            <span className="flex items-center justify-center gap-2">
              {judging ? <Loader2 size={16} className="animate-spin" /> : <Bot size={16} />}
              {judging ? 'AI 研判中...' : 'AI 一键判断'}
            </span>
          </CrystalButton>
          {judgeStatus && (
            <p className="text-xs text-center text-gray-500">{judgeStatus}</p>
          )}
        </div>
      )}
    </div>
  );
}
