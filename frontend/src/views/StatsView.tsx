import { useEffect } from 'react';
import { GlassCard } from '../components/ui/GlassCard';
import { useAppStore } from '../stores/appStore';
import { BarChart3, Database, Activity, TrendingUp } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

export function StatsView() {
  const { trainingHistory, setTrainingHistory } = useAppStore();

  useEffect(() => {
    fetch('/api/training/history')
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setTrainingHistory(data);
      })
      .catch(console.error);
  }, [setTrainingHistory]);

  const chartData = trainingHistory.map((run, i) => ({
    name: `#${run.id}`,
    accuracy: run.accuracy * 100,
    samples: run.sample_count,
    index: i,
  }));

  const totalSamples = trainingHistory.reduce((sum, r) => sum + r.sample_count, 0);
  const latestAccuracy = trainingHistory.length > 0
    ? trainingHistory[0].accuracy * 100
    : 0;

  return (
    <div className="h-full overflow-y-auto space-y-4">
      <h1 className="text-lg font-semibold text-gray-700 px-1">准确率看板</h1>

      {/* KPI Cards */}
      <div className="grid grid-cols-4 gap-4">
        {[
          { icon: BarChart3, label: '训练轮次', value: trainingHistory.length, color: 'blue' },
          { icon: Database, label: '总样本数', value: totalSamples, color: 'green' },
          { icon: Activity, label: '最新准确率', value: `${latestAccuracy.toFixed(1)}%`, color: 'purple' },
          { icon: TrendingUp, label: '模型版本', value: `v${trainingHistory.length}`, color: 'amber' },
        ].map(({ icon: Icon, label, value, color }) => (
          <GlassCard key={label} tilt className="p-5">
            <div className="flex items-center gap-3">
              <div className={`w-10 h-10 rounded-xl bg-${color}-100/50 flex items-center
                justify-center border border-${color}-200/30`}>
                <Icon size={20} className={`text-${color}-500`} />
              </div>
              <div>
                <p className="text-xs text-gray-400">{label}</p>
                <p className="text-xl font-bold text-gray-700">{value}</p>
              </div>
            </div>
          </GlassCard>
        ))}
      </div>

      {/* Accuracy chart */}
      <GlassCard className="p-6">
        <h2 className="text-sm font-semibold text-gray-600 mb-4">准确率趋势</h2>
        <div className="h-72">
          {chartData.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <defs>
                  <linearGradient id="colorAcc" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(0,0,0,0.06)" />
                <XAxis dataKey="name" tick={{ fontSize: 12 }} stroke="rgba(0,0,0,0.2)" />
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
                  dataKey="accuracy"
                  stroke="#3b82f6"
                  fillOpacity={1}
                  fill="url(#colorAcc)"
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-400 text-sm">
              暂无训练数据
            </div>
          )}
        </div>
      </GlassCard>
    </div>
  );
}
