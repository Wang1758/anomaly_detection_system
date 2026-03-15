import { useState, useCallback } from 'react';
import { GlassNavRail } from './GlassNavRail';
import { useAppStore } from '../../stores/appStore';
import { useWebSocket } from '../../hooks/useWebSocket';
import { MonitorView } from '../../views/MonitorView';
import { ConfigView } from '../../views/ConfigView';
import { StatsView } from '../../views/StatsView';
import { LibraryView } from '../../views/LibraryView';
import type { AlertEvent } from '../../types';

export function Shell() {
  const { currentView, addAlert } = useAppStore();
  const [spotlightPos, setSpotlightPos] = useState({ x: 0, y: 0 });

  const handleAlert = useCallback(
    (event: AlertEvent) => {
      addAlert(event);
    },
    [addAlert]
  );
  useWebSocket(handleAlert);

  const handleMouseMove = (e: React.MouseEvent) => {
    setSpotlightPos({ x: e.clientX, y: e.clientY });
  };

  const viewMap = {
    monitor: <MonitorView />,
    config: <ConfigView />,
    stats: <StatsView />,
    library: <LibraryView />,
  };

  return (
    <div
      className="h-screen w-screen mesh-gradient spotlight-container flex p-4 gap-4 overflow-hidden"
      onMouseMove={handleMouseMove}
    >
      <div
        className="spotlight"
        style={{ left: spotlightPos.x, top: spotlightPos.y }}
      />

      <GlassNavRail />

      <div className="flex-1 overflow-hidden">
        {viewMap[currentView]}
      </div>
    </div>
  );
}
