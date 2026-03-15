import { useState, useEffect, useMemo } from 'react';
import { GlassCard } from '../components/ui/GlassCard';
import { CrystalButton } from '../components/ui/CrystalButton';
import { FloatingBar } from '../components/ui/FloatingBar';
import { useAppStore } from '../stores/appStore';
import { RotateCcw, Save } from 'lucide-react';
import type { SystemConfig } from '../types';

interface SliderFieldProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (v: number) => void;
}

function SliderField({ label, value, min, max, step, onChange }: SliderFieldProps) {
  return (
    <div>
      <div className="flex justify-between mb-2">
        <span className="text-sm text-gray-600 font-medium">{label}</span>
        <span className="text-sm font-mono text-blue-600 bg-blue-50/50 px-2 py-0.5 rounded-lg">
          {value.toFixed(step < 1 ? 2 : 0)}
        </span>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(parseFloat(e.target.value))}
        className="w-full h-1.5 bg-gray-200/50 rounded-full appearance-none cursor-pointer
          [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4
          [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:rounded-full
          [&::-webkit-slider-thumb]:bg-blue-500 [&::-webkit-slider-thumb]:shadow-md
          [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-white"
      />
    </div>
  );
}

export function ConfigView() {
  const { config, setConfig } = useAppStore();
  const [local, setLocal] = useState<SystemConfig | null>(null);

  useEffect(() => {
    fetch('/api/config')
      .then((r) => r.json())
      .then((data) => {
        setConfig(data);
        setLocal(data);
      })
      .catch(console.error);
  }, [setConfig]);

  const isDirty = useMemo(() => {
    if (!config || !local) return false;
    return JSON.stringify(config) !== JSON.stringify(local);
  }, [config, local]);

  const handleSave = async () => {
    if (!local) return;
    try {
      await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(local),
      });
      setConfig(local);
    } catch (e) {
      console.error('Save failed:', e);
    }
  };

  const handleReset = () => {
    if (config) setLocal({ ...config });
  };

  if (!local) {
    return (
      <div className="h-full flex items-center justify-center text-gray-400">
        加载配置中...
      </div>
    );
  }

  const update = (field: keyof SystemConfig, value: number) => {
    setLocal({ ...local, [field]: value });
  };

  return (
    <div className="h-full overflow-y-auto pb-24 space-y-4">
      <h1 className="text-lg font-semibold text-gray-700 px-1">系统参数配置</h1>

      <div className="grid grid-cols-2 gap-4">
        {/* Python model params */}
        <GlassCard className="p-5 space-y-5">
          <h2 className="text-sm font-semibold text-gray-600 mb-1">AI 模型参数</h2>
          <SliderField label="NMS 阈值" value={local.nms_threshold} min={0} max={1} step={0.05}
            onChange={(v) => update('nms_threshold', v)} />
          <SliderField label="置信度阈值" value={local.confidence_threshold} min={0} max={1} step={0.05}
            onChange={(v) => update('confidence_threshold', v)} />
          <SliderField label="熵阈值" value={local.entropy_threshold} min={0} max={1} step={0.05}
            onChange={(v) => update('entropy_threshold', v)} />
          <SliderField label="W1 (置信度权重)" value={local.w1} min={0} max={1} step={0.05}
            onChange={(v) => update('w1', v)} />
          <SliderField label="W2 (拥挤度权重)" value={local.w2} min={0} max={1} step={0.05}
            onChange={(v) => update('w2', v)} />
        </GlassCard>

        {/* Go backend params */}
        <GlassCard className="p-5 space-y-5">
          <h2 className="text-sm font-semibold text-gray-600 mb-1">后端调度参数</h2>
          <SliderField label="时间窗口 (秒)" value={local.filter_time_window} min={5} max={300} step={5}
            onChange={(v) => update('filter_time_window', v)} />
          <SliderField label="IoU 阈值" value={local.filter_iou} min={0} max={1} step={0.05}
            onChange={(v) => update('filter_iou', v)} />
          <SliderField label="并发 Workers" value={local.workers} min={1} max={16} step={1}
            onChange={(v) => update('workers', v)} />
        </GlassCard>
      </div>

      <FloatingBar visible={isDirty}>
        <span className="text-sm text-amber-600 font-medium">有未保存的修改</span>
        <CrystalButton variant="danger" size="sm" onClick={handleReset}>
          <span className="flex items-center gap-1"><RotateCcw size={14} /> 重置</span>
        </CrystalButton>
        <CrystalButton variant="primary" size="sm" onClick={handleSave}>
          <span className="flex items-center gap-1"><Save size={14} /> 保存配置</span>
        </CrystalButton>
      </FloatingBar>
    </div>
  );
}
