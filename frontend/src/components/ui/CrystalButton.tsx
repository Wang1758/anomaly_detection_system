import { type ReactNode, type ButtonHTMLAttributes } from 'react';

type Variant = 'primary' | 'success' | 'danger';

interface CrystalButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  children: ReactNode;
  size?: 'sm' | 'md' | 'lg';
}

const sizeClasses = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-6 py-3 text-base',
};

export function CrystalButton({
  variant = 'primary',
  size = 'md',
  children,
  className = '',
  ...props
}: CrystalButtonProps) {
  return (
    <button
      className={`crystal-btn crystal-${variant} ${sizeClasses[size]} rounded-xl font-medium
        transition-all duration-300 active:scale-95 disabled:opacity-40 disabled:cursor-not-allowed
        ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}
