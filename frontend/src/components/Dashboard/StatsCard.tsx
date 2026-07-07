import { useEffect, useState } from 'react';
import type { LucideIcon } from 'lucide-react';

interface StatsCardProps {
  title: string;
  value: number;
  change?: string;
  icon: LucideIcon;
  color: string;
  delay?: number;
  format?: (value: number) => string;
}

export default function StatsCard({
  title,
  value,
  change,
  icon: Icon,
  color,
  delay = 0,
  format,
}: StatsCardProps) {
  const [displayValue, setDisplayValue] = useState(0);
  const isPositive = change ? change.startsWith('+') : true;

  // Guard against a missing/undefined value from the API so displayValue is always a
  // finite number — otherwise format() (e.g. v.toFixed) would throw and, without an
  // error boundary, take down the whole page.
  const safeValue = Number.isFinite(value) ? value : 0;

  useEffect(() => {
    const duration = 1200;
    const steps = 40;
    const stepDuration = duration / steps;
    let current = 0;

    const timeout = setTimeout(() => {
      const interval = setInterval(() => {
        current += 1;
        const progress = current / steps;
        // Ease-out cubic
        const eased = 1 - Math.pow(1 - progress, 3);
        setDisplayValue(Math.round(eased * safeValue));
        if (current >= steps) {
          clearInterval(interval);
          setDisplayValue(safeValue);
        }
      }, stepDuration);
      return () => clearInterval(interval);
    }, delay);

    return () => clearTimeout(timeout);
  }, [safeValue, delay]);

  const colorMap: Record<string, { border: string; bg: string; text: string; shadow: string }> = {
    blue: {
      border: 'border-l-[#3b82f6]',
      bg: 'bg-[#3b82f6]/10',
      text: 'text-[#3b82f6]',
      shadow: 'shadow-blue-500/20',
    },
    emerald: {
      border: 'border-l-[#10b981]',
      bg: 'bg-[#10b981]/10',
      text: 'text-[#10b981]',
      shadow: 'shadow-emerald-500/20',
    },
    amber: {
      border: 'border-l-[#f59e0b]',
      bg: 'bg-[#f59e0b]/10',
      text: 'text-[#f59e0b]',
      shadow: 'shadow-amber-500/20',
    },
    purple: {
      border: 'border-l-[#8b5cf6]',
      bg: 'bg-[#8b5cf6]/10',
      text: 'text-[#8b5cf6]',
      shadow: 'shadow-purple-500/20',
    },
    green: {
      border: 'border-l-[#22c55e]',
      bg: 'bg-[#22c55e]/10',
      text: 'text-[#22c55e]',
      shadow: 'shadow-green-500/20',
    },
  };

  const colors = colorMap[color] || colorMap.blue;

  return (
    <div
      className={`
        glass-card p-6 border-l-4 ${colors.border}
        hover:scale-[1.02] hover:shadow-xl ${colors.shadow}
        transition-all duration-300 cursor-default
        opacity-0 animate-fadeIn
      `}
      style={{ animationDelay: `${delay}ms` }}
    >
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm font-medium text-[#94a3b8]">{title}</span>
        <div className={`w-10 h-10 rounded-xl ${colors.bg} flex items-center justify-center`}>
          <Icon className={`w-5 h-5 ${colors.text}`} />
        </div>
      </div>
      <div className="flex items-end gap-3">
        <span className="text-3xl font-bold text-[#f8fafc] tabular-nums">
          {format ? format(displayValue) : displayValue.toLocaleString()}
        </span>
        {change && (
          <span
            className={`
              text-xs font-medium px-2 py-0.5 rounded-full mb-1
              ${
                isPositive
                  ? 'bg-emerald-500/15 text-emerald-400'
                  : 'bg-red-500/15 text-red-400'
              }
            `}
          >
            {change}
          </span>
        )}
      </div>
    </div>
  );
}
