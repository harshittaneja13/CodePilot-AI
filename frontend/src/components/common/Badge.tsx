import type { ReactNode } from 'react';

type BadgeVariant =
  | 'critical'
  | 'high'
  | 'medium'
  | 'low'
  | 'info'
  | 'completed'
  | 'failed'
  | 'pending'
  | 'in_progress'
  | 'open'
  | 'closed'
  | 'merged';

interface BadgeProps {
  variant: BadgeVariant;
  children: ReactNode;
  size?: 'sm' | 'md';
}

const variantStyles: Record<BadgeVariant, string> = {
  critical: 'bg-red-500/15 text-red-400 border-red-500/30',
  high: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
  medium: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
  low: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
  info: 'bg-slate-500/15 text-slate-400 border-slate-500/30',
  completed: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
  failed: 'bg-red-500/15 text-red-400 border-red-500/30',
  pending: 'bg-slate-500/15 text-slate-400 border-slate-500/30',
  in_progress: 'bg-blue-500/15 text-blue-400 border-blue-500/30',
  open: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
  closed: 'bg-red-500/15 text-red-400 border-red-500/30',
  merged: 'bg-purple-500/15 text-purple-400 border-purple-500/30',
};

export default function Badge({ variant, children, size = 'sm' }: BadgeProps) {
  const sizeClasses = size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-3 py-1 text-sm';

  return (
    <span
      className={`
        inline-flex items-center gap-1 font-medium rounded-full border
        transition-all duration-200
        ${variantStyles[variant]}
        ${sizeClasses}
      `}
    >
      {variant === 'in_progress' && (
        <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" />
      )}
      {variant === 'completed' && (
        <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
      )}
      {variant === 'failed' && (
        <span className="w-1.5 h-1.5 rounded-full bg-red-400" />
      )}
      {variant === 'pending' && (
        <span className="w-1.5 h-1.5 rounded-full bg-slate-400" />
      )}
      {children}
    </span>
  );
}
