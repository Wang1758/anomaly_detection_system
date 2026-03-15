import { TopControlBar } from '../components/monitor/TopControlBar';
import { LiveFeed } from '../components/monitor/LiveFeed';
import { PendingQueue } from '../components/monitor/PendingQueue';
import { Lightbox } from '../components/ui/Lightbox';
import { CrystalButton } from '../components/ui/CrystalButton';
import { useAppStore } from '../stores/appStore';
import { Check, X } from 'lucide-react';

export function MonitorView() {
  const { lightboxAlert, setLightboxAlert, removeAlert } = useAppStore();

  const handleLightboxLabel = async (label: boolean) => {
    if (!lightboxAlert) return;
    try {
      const res = await fetch(`/api/samples?status=pending`);
      const samples = await res.json();
      const sample = samples.find((s: { frame_id: number }) => s.frame_id === lightboxAlert.frame_id);
      if (sample) {
        await fetch(`/api/samples/${sample.id}/label`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ label }),
        });
      }
      removeAlert(lightboxAlert.frame_id);
      setLightboxAlert(null);
    } catch (e) {
      console.error('Label failed:', e);
    }
  };

  return (
    <div className="h-full flex flex-col gap-4">
      <TopControlBar />

      <div className="flex-1 flex gap-4 min-h-0">
        <LiveFeed />
        <PendingQueue />
      </div>

      {/* Lightbox for expanded alert view */}
      <Lightbox open={!!lightboxAlert} onClose={() => setLightboxAlert(null)}>
        {lightboxAlert && (
          <div>
            <img
              src={lightboxAlert.image_url}
              alt={`Frame ${lightboxAlert.frame_id}`}
              className="w-full rounded-xl mb-4"
            />
            <div className="flex items-center justify-between mb-4">
              <div>
                <span className="text-sm font-semibold text-gray-700">
                  Frame #{lightboxAlert.frame_id}
                </span>
                <p className="text-xs text-gray-400 mt-1">
                  {lightboxAlert.detections.filter((d) => d.is_uncertain).length} 个不确定目标 |{' '}
                  {lightboxAlert.timestamp}
                </p>
              </div>
            </div>

            <div className="flex gap-3 justify-center">
              <CrystalButton
                variant="success"
                size="lg"
                onClick={() => handleLightboxLabel(true)}
              >
                <span className="flex items-center gap-2">
                  <Check size={18} /> 确认 (正样本)
                </span>
              </CrystalButton>
              <CrystalButton
                variant="danger"
                size="lg"
                onClick={() => handleLightboxLabel(false)}
              >
                <span className="flex items-center gap-2">
                  <X size={18} /> 误报 (负样本)
                </span>
              </CrystalButton>
            </div>
          </div>
        )}
      </Lightbox>
    </div>
  );
}
