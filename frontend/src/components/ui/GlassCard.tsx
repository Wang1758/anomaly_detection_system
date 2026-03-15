import { type ReactNode } from 'react';
import { useTilt } from '../../hooks/useTilt';

interface GlassCardProps {
  children: ReactNode;
  className?: string;
  tilt?: boolean;
  onClick?: () => void;
}

export function GlassCard({ children, className = '', tilt = false, onClick }: GlassCardProps) {
  const { ref, onMouseMove, onMouseLeave } = useTilt<HTMLDivElement>({
    maxTilt: 3,
    scale: 1.01,
  });

  return (
    <div
      ref={tilt ? ref : undefined}
      onMouseMove={tilt ? onMouseMove : undefined}
      onMouseLeave={tilt ? onMouseLeave : undefined}
      onClick={onClick}
      className={`glass glow-border rounded-2xl ${onClick ? 'cursor-pointer' : ''} ${className}`}
    >
      {children}
    </div>
  );
}
