import { Eye, Settings, BarChart3, FolderOpen } from 'lucide-react';
import { useAppStore } from '../../stores/appStore';
import { useTilt } from '../../hooks/useTilt';
import type { ViewType } from '../../types';

const navItems: { id: ViewType; icon: typeof Eye; label: string }[] = [
  { id: 'monitor', icon: Eye, label: '监控' },
  { id: 'config', icon: Settings, label: '参数' },
  { id: 'stats', icon: BarChart3, label: '看板' },
  { id: 'library', icon: FolderOpen, label: '图库' },
];

export function GlassNavRail() {
  const { currentView, setCurrentView } = useAppStore();
  const { ref, onMouseMove, onMouseLeave } = useTilt<HTMLDivElement>({
    maxTilt: 5,
    scale: 1.01,
  });

  return (
    <div
      ref={ref}
      onMouseMove={onMouseMove}
      onMouseLeave={onMouseLeave}
      className="w-28 glass rounded-[32px] flex flex-col items-center py-9 gap-3 glow-border"
    >
      <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-blue-400/30 to-indigo-500/30
        flex items-center justify-center mb-6 border border-white/40">
        <Eye size={22} className="text-blue-600" />
      </div>

      <div className="flex flex-col gap-1 flex-1">
        {navItems.map(({ id, icon: Icon, label }) => {
          const active = currentView === id;
          return (
            <button
              key={id}
              onClick={() => setCurrentView(id)}
              className={`w-20 h-20 rounded-2xl flex flex-col items-center justify-center gap-1.5
                transition-all duration-300
                ${active
                  ? 'bg-white/70 shadow-lg shadow-blue-500/10 border border-white/60'
                  : 'hover:bg-white/40 border border-transparent'
                }`}
            >
              <Icon
                size={22}
                className={active ? 'text-blue-600' : 'text-gray-500'}
              />
              <span
                className={`text-xs font-medium ${
                  active ? 'text-blue-600' : 'text-gray-500'
                }`}
              >
                {label}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
