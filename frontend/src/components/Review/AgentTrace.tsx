import { useState } from 'react';
import {
  AlertTriangle,
  Bot,
  ChevronDown,
  ChevronRight,
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
import { getReviewLogs } from '@/lib/api';
import { prettyStep } from '@/lib/format';
import type { ExecutionLog } from '@/lib/types';

// Stable fallback so useApi's dependency identity doesn't change every render.
const EMPTY: ExecutionLog[] = [];

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

function iconFor(log: ExecutionLog): LucideIcon {
  if (log.status === 'failed') return AlertTriangle;
  return STEP_ICON[log.step] || Circle;
}

interface AgentTraceProps {
  reviewId: string;
}

/**
 * AgentTrace renders the review pipeline's execution log as a collapsible timeline —
 * triage, the agent's tool calls, reflection, and publishing — so you can see what the
 * agent actually did, with per-step status, duration, and messages.
 */
export default function AgentTrace({ reviewId }: AgentTraceProps) {
  const { data: logs, loading } = useApi<ExecutionLog[]>(
    () => getReviewLogs(reviewId),
    EMPTY,
    [reviewId],
  );
  const [open, setOpen] = useState(true);

  if (loading || !logs || logs.length === 0) {
    return null;
  }

  const totalMs = logs.reduce((sum, l) => sum + (l.duration_ms || 0), 0);

  return (
    <div className="mt-4 border-t border-[rgba(51,65,85,0.3)] pt-4">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2 w-full text-left mb-3"
      >
        {open ? (
          <ChevronDown className="w-4 h-4 text-[#64748b]" />
        ) : (
          <ChevronRight className="w-4 h-4 text-[#64748b]" />
        )}
        <h3 className="text-sm font-semibold text-[#f8fafc]">Agent Trace</h3>
        <span className="text-[11px] text-[#64748b] tabular-nums">
          {logs.length} steps · {(totalMs / 1000).toFixed(1)}s
        </span>
      </button>

      {open && (
        <ol className="relative border-l border-[#334155] ml-2 space-y-2.5">
          {logs.map((log) => {
            const Icon = iconFor(log);
            const failed = log.status === 'failed';
            const isTool = log.step === 'agent_tool';
            return (
              <li
                key={log.id}
                className={`ml-5 ${failed ? 'bg-red-500/5 -mx-1 px-1 py-1 rounded-md' : ''}`}
              >
                <span className="absolute -left-[9px] mt-0.5 bg-[#0f172a] rounded-full">
                  <Icon className={`w-3.5 h-3.5 ${STATUS_COLOR[log.status] || 'text-slate-400'}`} />
                </span>
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-xs font-medium text-[#f8fafc]">{prettyStep(log.step)}</span>
                  <span
                    className={`text-[10px] uppercase tracking-wide ${
                      STATUS_COLOR[log.status] || 'text-[#64748b]'
                    }`}
                  >
                    {log.status}
                  </span>
                  {log.duration_ms > 0 && (
                    <span className="text-[10px] text-[#64748b] tabular-nums">
                      {log.duration_ms}ms
                    </span>
                  )}
                </div>
                {log.message && (
                  <p
                    className={`mt-0.5 break-words ${
                      isTool
                        ? 'font-mono text-[10px] text-[#7c8db5]'
                        : 'text-xs text-[#94a3b8]'
                    }`}
                  >
                    {log.message}
                  </p>
                )}
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
