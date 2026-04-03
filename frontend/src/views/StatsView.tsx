import { useEffect, useMemo, useState } from 'react';
import { GlassCard } from '../components/ui/GlassCard';
import type { MapEvalRun } from '../types';
import { CrystalButton } from '../components/ui/CrystalButton';
import { RefreshCw } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

export function StatsView() {
  const [runs, setRuns] = useState<MapEvalRun[]>([]);
  const [evaluating, setEvaluating] = useState(false);
  const [hint, setHint] = useState<string | null>(null);
  const [evalLogs, setEvalLogs] = useState<string[]>([]);

  const loadEvalLogs = async () => {
    const res = await fetch('/api/metrics/map-eval-logs');
    if (!res.ok) return;
    const data = await res.json();
    if (Array.isArray(data?.logs)) {
      setEvalLogs(data.logs);
      if (typeof data?.running === 'boolean') {
        setEvaluating(data.running);
      }
    }
  };

  const loadHistory = async () => {
    const res = await fetch('/api/metrics/map-history');
    if (!res.ok) return;
    const data = await res.json();
    if (Array.isArray(data)) setRuns(data);
  };

  useEffect(() => {
    loadHistory().catch(console.error);
    loadEvalLogs().catch(console.error);

    const timer = window.setInterval(() => {
      loadEvalLogs().catch(console.error);
    }, 2000);

    return () => window.clearInterval(timer);
  }, []);

  const triggerEvalNow = async () => {
    setHint(null);
    setEvaluating(true);
    try {
      const res = await fetch('/api/metrics/map-eval-now', { method: 'POST' });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        setHint(data?.error || '触发失败');
        setEvaluating(false);
        return;
      }
      setHint('已触发评估，正在刷新结果...');

      const started = Date.now();
      const timer = window.setInterval(async () => {
        await loadHistory();
        if (Date.now() - started > 120000) {
          window.clearInterval(timer);
          setEvaluating(false);
          setHint('评估已触发，若耗时较长请稍后刷新');
        }
      }, 3000);

      window.setTimeout(() => {
        window.clearInterval(timer);
        setEvaluating(false);
      }, 30000);
    } catch (e) {
      console.error(e);
      setHint('网络错误，触发失败');
      setEvaluating(false);
    }
  };

  const chartData = useMemo(() => {
    return [...runs]
      .filter((r) => r.status === 'success')
      .reverse()
      .map((run) => ({
        time: new Date(run.created_at).toLocaleString('zh-CN', {
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
        }),
        map50: run.map50 * 100,
      }));
  }, [runs]);

  const latestMap = chartData.length > 0 ? chartData[chartData.length - 1].map50 : 0;

  return (
    <div className="h-full overflow-y-auto space-y-4">
      <h1 className="text-xl font-semibold text-gray-700 px-1">mAP 看板</h1>

      <GlassCard className="p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold text-gray-600">mAP@50 趋势</h2>
          <div className="flex items-center gap-2">
            <span className="text-sm font-mono text-blue-600 bg-blue-50/60 px-2 py-0.5 rounded-lg">
              最新 {latestMap.toFixed(2)}%
            </span>
            <CrystalButton size="sm" variant="primary" onClick={triggerEvalNow} disabled={evaluating}>
              <span className="flex items-center gap-1.5">
                <RefreshCw size={14} className={evaluating ? 'animate-spin' : ''} />
                立即评估一次
              </span>
            </CrystalButton>
          </div>
        </div>
        {hint && <p className="text-xs text-gray-500 mb-2">{hint}</p>}
        <div className="h-72">
          {chartData.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <defs>
                  <linearGradient id="colorMap" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(0,0,0,0.06)" />
                <XAxis dataKey="time" tick={{ fontSize: 12 }} stroke="rgba(0,0,0,0.2)" />
                <YAxis domain={[0, 100]} tick={{ fontSize: 12 }} stroke="rgba(0,0,0,0.2)" />
                <Tooltip
                  contentStyle={{
                    background: 'rgba(255,255,255,0.8)',
                    backdropFilter: 'blur(12px)',
                    border: '1px solid rgba(255,255,255,0.5)',
                    borderRadius: '12px',
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="map50"
                  stroke="#3b82f6"
                  fillOpacity={1}
                  fill="url(#colorMap)"
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-400 text-sm">
              暂无 mAP 评估数据
            </div>
          )}
        </div>
      </GlassCard>

      <GlassCard className="p-6">
        <h2 className="text-base font-semibold text-gray-600 mb-3">评估日志（后端实时）</h2>
        <div className="h-72 min-h-40 max-h-[75vh] resize-y overflow-auto rounded-xl bg-black/80 text-green-300 text-xs p-3 font-mono space-y-1">
          {evalLogs.length === 0 ? (
            <p className="text-gray-400">暂无评估日志</p>
          ) : (
            evalLogs.map((line, idx) => (
              <p key={`${idx}-${line.slice(0, 24)}`} className="whitespace-pre-wrap break-words">
                {line}
              </p>
            ))
          )}
        </div>
      </GlassCard>
    </div>
  );
}
