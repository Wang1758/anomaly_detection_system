import { useEffect, useState, useCallback } from 'react';
import { GlassCard } from '../components/ui/GlassCard';
import { CrystalButton } from '../components/ui/CrystalButton';
import { FloatingBar } from '../components/ui/FloatingBar';
import { useAppStore } from '../stores/appStore';
import { Zap, GripVertical } from 'lucide-react';
import type { Sample } from '../types';

export function LibraryView() {
  const { samples, setSamples } = useAppStore();
  const [dragItem, setDragItem] = useState<Sample | null>(null);

  const fetchSamples = useCallback(() => {
    fetch('/api/samples?status=labeled')
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setSamples(data);
      })
      .catch(console.error);
  }, [setSamples]);

  useEffect(() => {
    fetchSamples();
  }, [fetchSamples]);

  const positives = samples.filter((s) => s.label === true);
  const negatives = samples.filter((s) => s.label === false);

  const handleDragStart = (sample: Sample) => {
    setDragItem(sample);
  };

  const handleDrop = async (targetLabel: boolean) => {
    if (!dragItem || dragItem.label === targetLabel) {
      setDragItem(null);
      return;
    }

    try {
      await fetch(`/api/samples/${dragItem.id}/relabel`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label: targetLabel }),
      });
      fetchSamples();
    } catch (e) {
      console.error('Relabel failed:', e);
    }
    setDragItem(null);
  };

  const handleTriggerTraining = async () => {
    try {
      await fetch('/api/training/trigger', { method: 'POST' });
      fetchSamples();
    } catch (e) {
      console.error('Training trigger failed:', e);
    }
  };

  const renderContainer = (title: string, items: Sample[], isPositive: boolean, borderColor: string) => (
    <div
      className="flex-1 flex flex-col min-h-0"
      onDragOver={(e) => e.preventDefault()}
      onDrop={() => handleDrop(isPositive)}
    >
      <div className="flex items-center gap-2 mb-3 px-1">
        <div className={`w-3 h-3 rounded-full ${isPositive ? 'bg-green-400' : 'bg-red-400'}`} />
        <h2 className="text-base font-semibold text-gray-600">{title}</h2>
        <span className="text-sm text-gray-400 ml-auto">{items.length}</span>
      </div>

      <GlassCard className={`flex-1 overflow-y-auto p-3 border-2 border-dashed ${borderColor} transition-colors`}>
        <div className="grid grid-cols-3 gap-2">
          {items.map((sample) => (
            <div
              key={sample.id}
              draggable
              onDragStart={() => handleDragStart(sample)}
              className={`group relative rounded-xl overflow-hidden cursor-grab active:cursor-grabbing
                border-2 ${isPositive ? 'border-green-300/50' : 'border-red-300/50'}
                hover:shadow-lg transition-all`}
            >
              <img
                src={`/api/images/${sample.image_path.split('/').pop()}`}
                alt={`Sample ${sample.id}`}
                className="w-full h-28 object-cover"
              />
              <div className={`absolute inset-x-0 bottom-0 h-6 flex items-center justify-center
                ${isPositive ? 'bg-green-500/80' : 'bg-red-500/80'} text-white text-xs font-bold`}>
                {isPositive ? 'POSITIVE' : 'NEGATIVE'}
              </div>
              <div className="absolute top-1 left-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <GripVertical size={16} className="text-white drop-shadow-lg" />
              </div>
            </div>
          ))}
        </div>

        {items.length === 0 && (
          <div className="flex items-center justify-center h-32 text-gray-400 text-sm">
            拖拽图片到此处
          </div>
        )}
      </GlassCard>
    </div>
  );

  return (
    <div className="h-full flex flex-col">
      <h1 className="text-xl font-semibold text-gray-700 px-1 mb-4">已处理图片库</h1>

      <div className="flex-1 flex gap-4 min-h-0">
        {renderContainer('正样本 (确认)', positives, true, 'border-green-200/30')}
        {renderContainer('负样本 (误报)', negatives, false, 'border-red-200/30')}
      </div>

      <FloatingBar visible={samples.length > 0}>
        <span className="text-base text-gray-600">
          已标记 <span className="font-bold text-blue-600">{samples.length}</span> 个样本
        </span>
        <CrystalButton variant="primary" onClick={handleTriggerTraining}>
          <span className="flex items-center gap-2">
            <Zap size={18} /> 触发增量训练
          </span>
        </CrystalButton>
      </FloatingBar>
    </div>
  );
}
