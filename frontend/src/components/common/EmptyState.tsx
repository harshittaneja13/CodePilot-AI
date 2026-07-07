import type { LucideIcon } from 'lucide-react';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
}

export default function EmptyState({
  icon: Icon,
  title,
  description,
  actionLabel,
  onAction,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-20 animate-fadeIn">
      <div className="w-20 h-20 rounded-2xl bg-[#1e293b] border border-[#334155] flex items-center justify-center mb-6">
        <Icon className="w-10 h-10 text-[#64748b]" />
      </div>
      <h3 className="text-xl font-semibold text-[#f8fafc] mb-2">{title}</h3>
      <p className="text-[#94a3b8] text-sm max-w-md text-center mb-6">{description}</p>
      {actionLabel && onAction && (
        <button
          onClick={onAction}
          className="
            px-5 py-2.5 rounded-lg font-medium text-sm
            bg-[#3b82f6] text-white
            hover:bg-[#2563eb] hover:scale-105
            active:scale-95
            transition-all duration-200
            shadow-lg shadow-blue-500/25
          "
        >
          {actionLabel}
        </button>
      )}
    </div>
  );
}
