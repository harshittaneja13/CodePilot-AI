import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import type { ReviewActivity } from '@/lib/types';

interface TooltipPayloadItem {
  name: string;
  value: number;
  color: string;
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadItem[];
  label?: string;
}

function CustomTooltip({ active, payload, label }: CustomTooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="glass-card px-4 py-3 shadow-xl">
      <p className="text-xs font-medium text-[#94a3b8] mb-2">{label}</p>
      {payload.map((entry) => (
        <div key={entry.name} className="flex items-center gap-2 text-sm">
          <div
            className="w-2 h-2 rounded-full"
            style={{ backgroundColor: entry.color }}
          />
          <span className="text-[#94a3b8] capitalize">{entry.name}:</span>
          <span className="text-[#f8fafc] font-semibold">{entry.value}</span>
        </div>
      ))}
    </div>
  );
}

interface Props {
  activity: ReviewActivity[];
}

export default function ReviewChart({ activity }: Props) {
  const data = activity.slice(-7).map((item) => ({
    ...item,
    date: new Date(item.date).toLocaleDateString('en-US', {
      weekday: 'short',
    }),
  }));

  return (
    <div
      className="glass-card p-6 opacity-0 animate-fadeIn"
      style={{ animationDelay: '200ms' }}
    >
      <h3 className="text-base font-semibold text-[#f8fafc] mb-6">
        Review Activity
      </h3>
      <ResponsiveContainer width="100%" height={280}>
        <AreaChart data={data}>
          <defs>
            <linearGradient id="reviewGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3b82f6" stopOpacity={0.3} />
              <stop offset="100%" stopColor="#3b82f6" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="findingsGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#8b5cf6" stopOpacity={0.3} />
              <stop offset="100%" stopColor="#8b5cf6" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="#334155"
            vertical={false}
          />
          <XAxis
            dataKey="date"
            stroke="#64748b"
            fontSize={12}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            stroke="#64748b"
            fontSize={12}
            tickLine={false}
            axisLine={false}
          />
          <Tooltip content={<CustomTooltip />} />
          <Area
            type="monotone"
            dataKey="reviews"
            stroke="#3b82f6"
            strokeWidth={2}
            fill="url(#reviewGradient)"
          />
          <Area
            type="monotone"
            dataKey="findings"
            stroke="#8b5cf6"
            strokeWidth={2}
            fill="url(#findingsGradient)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
