import { useState, useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import {
  GitBranch,
  FileCode,
  Plus,
  Minus,
  Files,
  AlertTriangle,
  ShieldAlert,
  Info,
  Lightbulb,
  ChevronDown,
  ChevronRight,
  User,
  Clock,
  PlayCircle,
  Loader2,
} from 'lucide-react';
import Badge from '@/components/common/Badge';
import LoadingSkeleton from '@/components/common/LoadingSkeleton';
import { useApi } from '@/hooks/useApi';
import { getPullRequestById, getReviewById, getPullRequestFiles, triggerPullRequestReview, getAvailableModels } from '@/lib/api';
import { mockPullRequests } from '@/lib/mockData';
import AgentTrace from '@/components/Review/AgentTrace';
import type { ReviewComment, Severity, ChangedFile } from '@/lib/types';

const severityConfig: Record<
  Severity,
  { border: string; icon: typeof AlertTriangle; color: string; label: string }
> = {
  critical: { border: 'border-l-red-500', icon: ShieldAlert, color: 'text-red-400', label: 'Critical' },
  high: { border: 'border-l-amber-500', icon: AlertTriangle, color: 'text-amber-400', label: 'High' },
  medium: { border: 'border-l-blue-500', icon: Info, color: 'text-blue-400', label: 'Medium' },
  low: { border: 'border-l-emerald-500', icon: Lightbulb, color: 'text-emerald-400', label: 'Low' },
  info: { border: 'border-l-slate-500', icon: Info, color: 'text-slate-400', label: 'Info' },
};

// Stable module-level fallbacks — prevent useApi's useCallback from re-running on every render.
const mockPrData = {
  pull_request: mockPullRequests[0],
  reviews: [] as { id: string }[],
};
const FALLBACK_MODELS = ['llama-3.3-70b-versatile'];

function CommentCard({ comment }: { comment: ReviewComment }) {
  const [expanded, setExpanded] = useState(true);
  const config = severityConfig[comment.severity];
  const SeverityIcon = config.icon;

  return (
    <div className={`glass-card border-l-4 ${config.border} overflow-hidden transition-all duration-300 hover:shadow-lg`}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-5 py-4 flex items-start gap-3 text-left hover:bg-[rgba(255,255,255,0.02)] transition-colors"
      >
        <SeverityIcon className={`w-5 h-5 mt-0.5 ${config.color} shrink-0`} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant={comment.severity}>{config.label}</Badge>
          </div>
          <h4 className="text-sm font-semibold text-[#f8fafc] mt-2">{comment.title}</h4>
          {comment.file_path && (
            <p className="text-xs text-[#64748b] font-mono mt-1">
              {comment.file_path}{comment.line_number ? `:${comment.line_number}` : ''}
            </p>
          )}
        </div>
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-[#64748b] shrink-0 mt-1" />
        ) : (
          <ChevronRight className="w-4 h-4 text-[#64748b] shrink-0 mt-1" />
        )}
      </button>

      {expanded && (
        <div className="px-5 pb-5 space-y-4 animate-fadeIn">
          <p className="text-sm text-[#cbd5e1] leading-relaxed">{comment.explanation}</p>

          {comment.why_it_matters && (
            <div className="bg-[rgba(59,130,246,0.05)] border border-[rgba(59,130,246,0.15)] rounded-lg p-4">
              <h5 className="text-xs font-semibold text-[#60a5fa] uppercase tracking-wider mb-2">
                Why it matters
              </h5>
              <p className="text-sm text-[#94a3b8] leading-relaxed">{comment.why_it_matters}</p>
            </div>
          )}

          {comment.code_snippet && (
            <div>
              <h5 className="text-xs font-semibold text-[#94a3b8] uppercase tracking-wider mb-2">
                Code
              </h5>
              <pre className="bg-[#0f172a] rounded-lg p-4 overflow-x-auto border border-[rgba(51,65,85,0.5)]">
                <code className="text-xs text-red-300 font-mono whitespace-pre-wrap">
                  {comment.code_snippet}
                </code>
              </pre>
            </div>
          )}

          {comment.suggestion && (
            <div className="bg-[rgba(16,185,129,0.05)] border border-[rgba(16,185,129,0.15)] rounded-lg p-4">
              <h5 className="text-xs font-semibold text-emerald-400 uppercase tracking-wider mb-2">
                Suggestion
              </h5>
              <p className="text-sm text-[#94a3b8] leading-relaxed">{comment.suggestion}</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function DiffLine({ line }: { line: string }) {
  if (line.startsWith('@@')) {
    return (
      <div className="px-3 py-0.5 bg-[rgba(59,130,246,0.08)] text-[#60a5fa] text-xs font-mono">
        {line}
      </div>
    );
  }
  if (line.startsWith('+')) {
    return (
      <div className="px-3 py-0.5 bg-[rgba(16,185,129,0.08)] text-emerald-300 text-xs font-mono whitespace-pre">
        {line}
      </div>
    );
  }
  if (line.startsWith('-')) {
    return (
      <div className="px-3 py-0.5 bg-[rgba(239,68,68,0.08)] text-red-300 text-xs font-mono whitespace-pre">
        {line}
      </div>
    );
  }
  return (
    <div className="px-3 py-0.5 text-[#64748b] text-xs font-mono whitespace-pre">
      {line}
    </div>
  );
}

function FileRow({ file }: { file: ChangedFile }) {
  const [expanded, setExpanded] = useState(false);
  const lines = file.patch ? file.patch.split('\n') : [];

  return (
    <div className="rounded-lg border border-[rgba(51,65,85,0.4)] overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 py-2.5 px-3 hover:bg-[rgba(255,255,255,0.02)] transition-colors text-left"
      >
        {expanded
          ? <ChevronDown className="w-3.5 h-3.5 text-[#64748b] shrink-0" />
          : <ChevronRight className="w-3.5 h-3.5 text-[#64748b] shrink-0" />}
        <FileCode className="w-4 h-4 text-[#64748b] shrink-0" />
        <span className="text-sm text-[#e2e8f0] font-mono truncate flex-1">{file.filename}</span>
        <Badge
          variant={
            file.status === 'added' ? 'completed' : file.status === 'deleted' ? 'failed' : 'medium'
          }
        >
          {file.status}
        </Badge>
        <span className="text-xs text-emerald-400 font-mono ml-2">+{file.additions}</span>
        <span className="text-xs text-red-400 font-mono ml-1">-{file.deletions}</span>
      </button>

      {expanded && (
        <div className="border-t border-[rgba(51,65,85,0.4)] overflow-x-auto max-h-[400px] overflow-y-auto bg-[#0a0f1a]">
          {lines.length > 0 ? (
            lines.map((line, i) => <DiffLine key={i} line={line} />)
          ) : (
            <p className="px-3 py-4 text-xs text-[#64748b]">No diff available for this file.</p>
          )}
        </div>
      )}
    </div>
  );
}

export default function PullRequestDetail() {
  const { id } = useParams<{ id: string }>();
  const [triggering, setTriggering] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [selectedModel, setSelectedModel] = useState(FALLBACK_MODELS[0]);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const { data: availableModels } = useApi(getAvailableModels, FALLBACK_MODELS);

  // When the models list loads, keep the current selection if it's valid.
  // If the loaded list doesn't include the current selection, switch to the first available.
  useEffect(() => {
    if (availableModels && availableModels.length > 0 && !availableModels.includes(selectedModel)) {
      setSelectedModel(availableModels[0]);
    }
  }, [availableModels]); // eslint-disable-line react-hooks/exhaustive-deps

  const { data: prData, loading: prLoading, silentRefetch: silentRefetchPr } = useApi(
    () => getPullRequestById(id!),
    mockPrData,
    [id],
  );

  const firstReviewId = prData?.reviews?.[0]?.id;

  const { data: reviewData, silentRefetch: silentRefetchReview } = useApi(
    () => firstReviewId ? getReviewById(firstReviewId) : Promise.resolve(null),
    undefined,
    [firstReviewId],
  );

  const { data: files } = useApi(
    () => getPullRequestFiles(id!),
    undefined,
    [id],
  );

  const reviewStatus = reviewData?.review?.status;

  // Stop polling once the review reaches a terminal state.
  useEffect(() => {
    if (pollRef.current && (reviewStatus === 'completed' || reviewStatus === 'failed')) {
      stopPolling();
      setReviewing(false);
    }
  }, [reviewStatus]);

  // Clean up on unmount.
  useEffect(() => () => stopPolling(), []);

  function stopPolling() {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }

  async function handleTriggerReview() {
    if (!id) return;
    setTriggering(true);
    try {
      const model = selectedModel;
      await triggerPullRequestReview(id, model);
      setReviewing(true);
      // Poll PR (to pick up the new review ID) and review details (to track status).
      pollRef.current = setInterval(() => {
        silentRefetchPr();
        silentRefetchReview();
      }, 4000);
    } finally {
      setTriggering(false);
    }
  }

  const [activeTab, setActiveTab] = useState<'overview' | 'comments'>('comments');

  if (prLoading || !prData) {
    return <LoadingSkeleton type="text" count={3} />;
  }

  const pr = prData.pull_request;
  const review = reviewData?.review ?? null;
  const comments = reviewData?.comments ?? [];
  const reviewFailed = review?.status === 'failed';
  const reviewCompleted = review?.status === 'completed';
  const reviewInProgress = reviewing || review?.status === 'in_progress' || review?.status === 'pending';
  // Allow running/re-running when: no review, review failed, or review completed (new commits may exist)
  const canRunReview = !reviewInProgress;

  const tabs = [
    { key: 'overview' as const, label: 'Overview' },
    { key: 'comments' as const, label: `Review Comments (${comments.length})` },
  ];

  return (
    <div className="space-y-6 max-w-5xl animate-fadeIn">
      {/* Header */}
      <div className="flex items-start gap-4">
        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-[#3b82f6] to-[#8b5cf6] flex items-center justify-center shrink-0 shadow-lg shadow-blue-500/20">
          <User className="w-6 h-6 text-white" />
        </div>
        <div className="flex-1 min-w-0">
          <h1 className="text-xl font-bold text-[#f8fafc] leading-tight">{pr.title}</h1>
          <div className="flex items-center gap-3 mt-2 flex-wrap">
            <span className="text-sm text-[#64748b] font-mono">#{pr.github_number}</span>
            <span className="text-sm text-[#94a3b8]">by</span>
            <span className="text-sm font-medium text-[#e2e8f0]">{pr.author}</span>
            <Badge variant={pr.state}>{pr.state}</Badge>
          </div>
          <div className="flex items-center gap-2 mt-3">
            <span className="inline-flex items-center gap-1 px-2.5 py-1 bg-[#1e293b] border border-[#334155] rounded-md text-xs font-mono text-[#60a5fa]">
              <GitBranch className="w-3 h-3" />
              {pr.head_branch}
            </span>
            <span className="text-[#64748b]">→</span>
            <span className="inline-flex items-center gap-1 px-2.5 py-1 bg-[#1e293b] border border-[#334155] rounded-md text-xs font-mono text-[#94a3b8]">
              <GitBranch className="w-3 h-3" />
              {pr.base_branch}
            </span>
          </div>
        </div>
      </div>

      {/* Stats row */}
      <div className="flex items-center gap-6">
        <div className="flex items-center gap-2 text-sm">
          <Plus className="w-4 h-4 text-emerald-400" />
          <span className="font-semibold text-emerald-400">{pr.additions.toLocaleString()}</span>
          <span className="text-[#64748b]">additions</span>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <Minus className="w-4 h-4 text-red-400" />
          <span className="font-semibold text-red-400">{pr.deletions.toLocaleString()}</span>
          <span className="text-[#64748b]">deletions</span>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <Files className="w-4 h-4 text-[#94a3b8]" />
          <span className="font-semibold text-[#e2e8f0]">{pr.changed_files}</span>
          <span className="text-[#64748b]">files changed</span>
        </div>
      </div>

      {/* Model picker + Run Review — shown when no review or previous review failed */}
      {!reviewInProgress && canRunReview && (
        <div className="glass-card p-5 space-y-3">
          {reviewFailed && (
            <div className="flex items-center gap-2 text-red-400 text-xs font-medium">
              <span className="w-2 h-2 rounded-full bg-red-500 shrink-0" />
              Previous review failed — select a model and try again
            </div>
          )}
          {reviewCompleted && (
            <div className="flex items-center gap-2 text-[#64748b] text-xs">
              <span className="w-2 h-2 rounded-full bg-emerald-500 shrink-0" />
              Review completed — re-run if new commits have been pushed
            </div>
          )}
          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex-1 min-w-[200px]">
              <label className="text-xs text-[#64748b] mb-1 block">Model</label>
              <select
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
                className="w-full bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2 text-sm text-[#f8fafc] focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30 transition-all"
              >
                {(availableModels ?? ['llama-3.3-70b-versatile']).map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
            </div>
            <div className="pt-5">
              <button
                onClick={handleTriggerReview}
                disabled={triggering}
                className="flex items-center gap-2 px-5 py-2 rounded-lg text-sm font-medium bg-[#3b82f6] text-white hover:bg-[#2563eb] disabled:opacity-50 transition-all duration-200 whitespace-nowrap"
              >
                {triggering ? <Loader2 className="w-4 h-4 animate-spin" /> : <PlayCircle className="w-4 h-4" />}
                {triggering ? 'Queuing…' : (reviewFailed || reviewCompleted) ? 'Re-run Review' : 'Run Review'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Review summary — only shown when a completed review exists AND not currently re-running */}
      {review && reviewCompleted && !reviewInProgress ? (
        <div className="glass-card p-5">
          <div className="flex items-center gap-6 flex-wrap">
            {review.llm_model && (
              <div className="flex items-center gap-2">
                <span className="text-sm text-[#94a3b8]">Model:</span>
                <span className="text-sm font-medium text-[#f8fafc] bg-[#1e293b] px-2.5 py-0.5 rounded-md border border-[#334155]">
                  {review.llm_model}
                </span>
              </div>
            )}
            <div className="flex items-center gap-3">
              {review.critical_count > 0 && (
                <div className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-red-500" />
                  <span className="text-xs text-[#94a3b8]">{review.critical_count} Critical</span>
                </div>
              )}
              {review.high_count > 0 && (
                <div className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-amber-500" />
                  <span className="text-xs text-[#94a3b8]">{review.high_count} High</span>
                </div>
              )}
              {review.medium_count > 0 && (
                <div className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-blue-500" />
                  <span className="text-xs text-[#94a3b8]">{review.medium_count} Medium</span>
                </div>
              )}
              {review.low_count > 0 && (
                <div className="flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-emerald-500" />
                  <span className="text-xs text-[#94a3b8]">{review.low_count} Low</span>
                </div>
              )}
            </div>
            <div className="flex items-center gap-4 text-xs text-[#64748b] ml-auto">
              {review.cost_usd > 0 && (
                <span>
                  Cost:{' '}
                  <span className="text-[#f8fafc] font-medium tabular-nums">
                    ${review.cost_usd.toFixed(4)}
                  </span>
                </span>
              )}
              {review.tokens_used > 0 && (
                <span className="tabular-nums">
                  {review.tokens_used.toLocaleString()} tokens
                  {review.input_tokens > 0 && (
                    <span className="text-[#475569]">
                      {' '}
                      ({review.input_tokens.toLocaleString()} in /{' '}
                      {review.output_tokens.toLocaleString()} out)
                    </span>
                  )}
                </span>
              )}
              {review.processing_time_ms > 0 && (
                <span>Reviewed in {(review.processing_time_ms / 1000).toFixed(1)}s</span>
              )}
            </div>
          </div>
          {review.summary && (
            <p className="text-sm text-[#94a3b8] mt-3 border-t border-[rgba(51,65,85,0.3)] pt-3">
              {review.summary}
            </p>
          )}
          <AgentTrace reviewId={review.id} />
        </div>
      ) : reviewInProgress ? (
        <div className="glass-card p-6">
          <div className="flex flex-col items-center gap-4 py-2">
            {/* Animated ring */}
            <div className="relative w-14 h-14">
              <div className="absolute inset-0 rounded-full border-4 border-[#1e293b]" />
              <div className="absolute inset-0 rounded-full border-4 border-t-[#3b82f6] animate-spin" />
              <div className="absolute inset-0 flex items-center justify-center">
                <PlayCircle className="w-5 h-5 text-[#3b82f6]" />
              </div>
            </div>
            <div className="text-center">
              <p className="text-sm font-semibold text-[#f8fafc]">AI Review in Progress</p>
              <p className="text-xs text-[#64748b] mt-1">
                Analysing code, checking for issues… this usually takes 30–90 seconds.
              </p>
            </div>
            {/* Pulse dots */}
            <div className="flex items-center gap-1.5">
              {[0, 1, 2].map((i) => (
                <span
                  key={i}
                  className="w-1.5 h-1.5 rounded-full bg-[#3b82f6] animate-bounce"
                  style={{ animationDelay: `${i * 0.15}s` }}
                />
              ))}
            </div>
          </div>
        </div>
      ) : null}

      {/* Tabs */}
      <div className="flex gap-1 border-b border-[rgba(51,65,85,0.5)]">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`
              px-4 py-3 text-sm font-medium transition-all duration-200 border-b-2 -mb-px
              ${
                activeTab === tab.key
                  ? 'text-[#3b82f6] border-[#3b82f6]'
                  : 'text-[#64748b] border-transparent hover:text-[#94a3b8] hover:border-[#334155]'
              }
            `}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <div className="space-y-4 animate-fadeIn">
          {pr.body && (
            <div className="glass-card p-5">
              <h3 className="text-sm font-semibold text-[#f8fafc] mb-3">Description</h3>
              <p className="text-sm text-[#94a3b8] leading-relaxed whitespace-pre-wrap">{pr.body}</p>
            </div>
          )}

          <div className="glass-card p-5">
            <h3 className="text-sm font-semibold text-[#f8fafc] mb-3">
              Changed Files ({files?.length ?? pr.changed_files})
            </h3>
            {files && files.length > 0 ? (
              <div className="space-y-2">
                {files.map((file) => (
                  <FileRow key={file.filename} file={file} />
                ))}
              </div>
            ) : (
              <p className="text-sm text-[#64748b]">Loading files…</p>
            )}
          </div>
        </div>
      )}

      {activeTab === 'comments' && (
        <div className="space-y-4 animate-fadeIn">
          {comments.length === 0 ? (
            <div className="glass-card p-12 text-center text-[#64748b] text-sm">
              {review ? 'No review comments found.' : 'No AI review has been run on this PR yet.'}
            </div>
          ) : (
            comments.map((comment) => (
              <CommentCard key={comment.id} comment={comment} />
            ))
          )}
        </div>
      )}
    </div>
  );
}
