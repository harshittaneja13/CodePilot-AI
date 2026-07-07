import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Activity,
  AlertTriangle,
  Bot,
  Circle,
  Database,
  FileText,
  Layers,
  ListFilter,
  MessageSquareCode,
  Send,
  ShieldCheck,
  Wrench,
  type LucideIcon,
} from 'lucide-react';
import { useApi } from '@/hooks/useApi';
import { getActivity } from '@/lib/api';
import { formatTimeAgo, prettyStep } from '@/lib/format';
import { mockActivity } from '@/lib/mockData';
import type { ActivityEvent } from '@/lib/types';

const STEP_ICON: Record<string, LucideIcon> = {
  triage: ListFilter,
  agent_review: Bot,
  agent_tool: Wrench,
  llm_review: MessageSquareCode,
  reflection: ShieldCheck,
  rag_index: Database,
  fetch_files: FileText,
  build_context: Layers,
  publish_review: Send,
};

const STATUS_COLOR: Record<string, string> = {
  success: 'text-emerald-400',
  failed: 'text-red-400',
  skipped: 'text-slate-400',
  info: 'text-blue-400',
};

function iconFor(ev: ActivityEvent): LucideIcon {
  if (ev.status === 'failed') return AlertTriangle;
  return STEP_ICON[ev.step] || Circle;
}

/**
 * ActivityFeed streams recent pipeline milestones across all reviews and polls every
 * 5s for near-real-time updates. Clicking an event opens the corresponding PR.
 */
export default function ActivityFeed() {
  const navigate = useNavigate();
  const { data, silentRefetch } = useApi<ActivityEvent[]>(
    () => getActivity(30),
    mockActivity,
    [],
  );

  useEffect(() => {
    const id = setInterval(silentRefetch, 5000);
    return () => clearInterval(id);
  }, [silentRefetch]);

  const events = data ?? [];

  return (
    <div
      className="glass-card overflow-hidden animate-fadeIn xl:sticky xl:top-8"
      style={{ animationDelay: '250ms' }}
    >
      <div className="px-5 py-4 border-b border-[rgba(51,65,85,0.5)] flex items-center gap-2">
        <Activity className="w-4 h-4 text-[#3b82f6]" />
        <h3 className="text-base font-semibold text-[#f8fafc]">Live Activity</h3>
        <span className="ml-auto flex items-center gap-1.5 text-[10px] text-[#64748b] uppercase tracking-wide">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse-glow" />
          live
        </span>
      </div>

      <div className="max-h-[560px] overflow-y-auto divide-y divide-[rgba(51,65,85,0.2)]">
        {events.length === 0 ? (
          <p className="px-5 py-8 text-center text-sm text-[#64748b]">No recent activity.</p>
        ) : (
          events.map((ev) => {
            const Icon = iconFor(ev);
            return (
              <button
                key={ev.id}
                onClick={() => navigate(`/pull-requests/${ev.pull_request_id}`)}
                className="w-full text-left px-5 py-3 hover:bg-[rgba(59,130,246,0.05)] transition-colors flex gap-3"
              >
                <Icon
                  className={`w-4 h-4 mt-0.5 shrink-0 ${STATUS_COLOR[ev.status] || 'text-slate-400'}`}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5 text-xs">
                    <span className="font-medium text-[#e2e8f0] capitalize">
                      {prettyStep(ev.step)}
                    </span>
                    <span className="text-[#64748b] font-mono truncate">
                      {ev.repository_name} #{ev.pr_number}
                    </span>
                  </div>
                  {ev.message && (
                    <p className="text-[11px] text-[#94a3b8] mt-0.5 truncate">{ev.message}</p>
                  )}
                  <span className="text-[10px] text-[#475569]">{formatTimeAgo(ev.created_at)}</span>
                </div>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}
