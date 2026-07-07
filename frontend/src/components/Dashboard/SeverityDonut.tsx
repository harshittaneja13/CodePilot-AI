import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Tooltip,
} from 'recharts';
import type { DashboardStats } from '@/lib/types';

const SEVERITY_COLORS: Record<string, string> = {
  Critical: '#dc2626',
  High: '#f59e0b',
  Medium: '#3b82f6',
  Low: '#10b981',
};

interface TooltipPayloadItem {
  name: string;
  value: number;
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: { payload: TooltipPayloadItem }[];
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload?.length) return null;
  const item = payload[0].payload;
  return (
    <div className="glass-card px-3 py-2 shadow-xl">
      <div className="flex items-center gap-2 text-sm">
        <div
          className="w-2 h-2 rounded-full"
          style={{ backgroundColor: SEVERITY_COLORS[item.name] }}
        />
        <span className="text-[#94a3b8]">{item.name}:</span>
        <span className="text-[#f8fafc] font-semibold">{item.value}</span>
      </div>
    </div>
  );
}

interface Props {
  stats: Pick<DashboardStats, 'critical_findings' | 'high_findings' | 'medium_findings' | 'low_findings'>;
}

export default function SeverityDonut({ stats }: Props) {

  const data = [
    { name: 'Critical', value: stats.critical_findings },
    { name: 'High', value: stats.high_findings },
    { name: 'Medium', value: stats.medium_findings },
    { name: 'Low', value: stats.low_findings },
  ];

  const total = data.reduce((sum, d) => sum + d.value, 0);

  return (
    <div
      className="glass-card p-6 opacity-0 animate-fadeIn"
      style={{ animationDelay: '250ms' }}
    >
      <h3 className="text-base font-semibold text-[#f8fafc] mb-6">
        Findings by Severity
      </h3>
      <div className="relative">
        <ResponsiveContainer width="100%" height={220}>
          <PieChart>
            <Pie
              data={data}
              cx="50%"
              cy="50%"
              innerRadius={60}
              outerRadius={85}
              paddingAngle={3}
              dataKey="value"
              stroke="none"
            >
              {data.map((entry) => (
                <Cell
                  key={entry.name}
                  fill={SEVERITY_COLORS[entry.name]}
                  className="transition-all duration-300 hover:opacity-80"
                />
              ))}
            </Pie>
            <Tooltip content={<CustomTooltip />} />
          </PieChart>
        </ResponsiveContainer>
        {/* Center text */}
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none" style={{ marginBottom: '36px' }}>
          <div className="text-center">
            <span className="text-2xl font-bold text-[#f8fafc]">
              {total.toLocaleString()}
            </span>
            <p className="text-[10px] text-[#64748b] uppercase tracking-wider">
              Total
            </p>
          </div>
        </div>
      </div>
      {/* Legend */}
      <div className="grid grid-cols-2 gap-2 mt-2">
        {data.map((item) => (
          <div key={item.name} className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-sm"
              style={{ backgroundColor: SEVERITY_COLORS[item.name] }}
            />
            <span className="text-xs text-[#94a3b8]">{item.name}</span>
            <span className="text-xs font-semibold text-[#e2e8f0] ml-auto">
              {item.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
