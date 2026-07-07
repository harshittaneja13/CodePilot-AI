import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  SlidersHorizontal,
  ChevronLeft,
  ChevronRight,
  ArrowUpDown,
} from 'lucide-react';
import Badge from '@/components/common/Badge';
import { useApi } from '@/hooks/useApi';
import { getDashboardStats } from '@/lib/api';
import type { DashboardReview, ReviewStatus } from '@/lib/types';

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatStatus(status: string): string {
  return status.replace('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

const PAGE_SIZE = 10;

export default function Reviews() {
  const navigate = useNavigate();
  const { data: stats } = useApi(getDashboardStats);
  const allReviews: DashboardReview[] = stats?.recent_reviews ?? [];

  const [statusFilter, setStatusFilter] = useState<ReviewStatus | 'all'>('all');
  const [repoFilter, setRepoFilter] = useState<string>('all');
  const [sortAsc, setSortAsc] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);

  const repos = useMemo(
    () => [...new Set(allReviews.map((r) => r.repository_name))],
    [allReviews]
  );

  const filteredReviews = useMemo(() => {
    let result = [...allReviews];
    if (statusFilter !== 'all') result = result.filter((r) => r.status === statusFilter);
    if (repoFilter !== 'all') result = result.filter((r) => r.repository_name === repoFilter);
    result.sort((a, b) => {
      const diff = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
      return sortAsc ? diff : -diff;
    });
    return result;
  }, [allReviews, statusFilter, repoFilter, sortAsc]);

  const totalPages = Math.max(1, Math.ceil(filteredReviews.length / PAGE_SIZE));
  const paginatedReviews = filteredReviews.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE
  );

  return (
    <div className="space-y-6 max-w-6xl animate-fadeIn">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-[#f8fafc]">Reviews</h1>
        <p className="text-[#94a3b8] text-sm mt-1">
          All AI-powered code reviews across your repositories
        </p>
      </div>

      {/* Filter bar */}
      <div className="glass-card px-5 py-4 flex items-center gap-4 flex-wrap">
        <SlidersHorizontal className="w-4 h-4 text-[#64748b]" />

        <select
          value={statusFilter}
          onChange={(e) => {
            setStatusFilter(e.target.value as ReviewStatus | 'all');
            setCurrentPage(1);
          }}
          className="
            bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2
            text-sm text-[#f8fafc]
            focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30
            transition-all
          "
        >
          <option value="all">All Statuses</option>
          <option value="completed">Completed</option>
          <option value="in_progress">In Progress</option>
          <option value="failed">Failed</option>
          <option value="pending">Pending</option>
        </select>

        <select
          value={repoFilter}
          onChange={(e) => {
            setRepoFilter(e.target.value);
            setCurrentPage(1);
          }}
          className="
            bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2
            text-sm text-[#f8fafc]
            focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30
            transition-all
          "
        >
          <option value="all">All Repositories</option>
          {repos.map((repo) => (
            <option key={repo} value={repo}>
              {repo}
            </option>
          ))}
        </select>

        <button
          onClick={() => setSortAsc(!sortAsc)}
          className="
            flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm
            text-[#94a3b8] hover:text-[#f8fafc] hover:bg-[#1e293b]
            border border-[#334155]
            transition-all duration-200
          "
        >
          <ArrowUpDown className="w-3.5 h-3.5" />
          {sortAsc ? 'Oldest First' : 'Newest First'}
        </button>

        <span className="text-xs text-[#64748b] ml-auto">
          {filteredReviews.length} review{filteredReviews.length !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Table */}
      <div className="glass-card overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[rgba(51,65,85,0.5)]">
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                  Repository
                </th>
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                  Pull Request
                </th>
                <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                  Status
                </th>
                <th className="text-center text-xs font-medium text-[#64748b] uppercase tracking-wider px-3 py-3">
                  <span className="text-red-400">Crit</span>
                </th>
                <th className="text-center text-xs font-medium text-[#64748b] uppercase tracking-wider px-3 py-3">
                  <span className="text-amber-400">High</span>
                </th>
                <th className="text-center text-xs font-medium text-[#64748b] uppercase tracking-wider px-3 py-3">
                  <span className="text-blue-400">Med</span>
                </th>
                <th className="text-center text-xs font-medium text-[#64748b] uppercase tracking-wider px-3 py-3">
                  <span className="text-emerald-400">Low</span>
                </th>
                <th className="text-right text-xs font-medium text-[#64748b] uppercase tracking-wider px-4 py-3">
                  Cost
                </th>
                <th className="text-right text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                  Date
                </th>
              </tr>
            </thead>
            <tbody>
              {paginatedReviews.map((review) => (
                <tr
                  key={review.id}
                  onClick={() =>
                    navigate(`/pull-requests/${review.pull_request_id}`)
                  }
                  className="
                    border-b border-[rgba(51,65,85,0.2)] cursor-pointer
                    hover:bg-[rgba(59,130,246,0.05)] transition-colors duration-150
                  "
                >
                  <td className="px-6 py-4">
                    <span className="text-sm font-medium text-[#e2e8f0]">
                      {review.repository_name}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex flex-col">
                      <span className="text-sm text-[#f8fafc] font-medium truncate max-w-[220px]">
                        {review.pr_title}
                      </span>
                      <span className="text-xs text-[#64748b] font-mono">
                        #{review.pr_number}
                      </span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <Badge variant={review.status}>
                      {formatStatus(review.status)}
                    </Badge>
                  </td>
                  <td className="px-3 py-4 text-center">
                    <span
                      className={`text-sm font-semibold ${
                        review.critical_count > 0
                          ? 'text-red-400'
                          : 'text-[#475569]'
                      }`}
                    >
                      {review.critical_count}
                    </span>
                  </td>
                  <td className="px-3 py-4 text-center">
                    <span
                      className={`text-sm font-semibold ${
                        review.high_count > 0
                          ? 'text-amber-400'
                          : 'text-[#475569]'
                      }`}
                    >
                      {review.high_count}
                    </span>
                  </td>
                  <td className="px-3 py-4 text-center">
                    <span
                      className={`text-sm font-semibold ${
                        review.medium_count > 0
                          ? 'text-blue-400'
                          : 'text-[#475569]'
                      }`}
                    >
                      {review.medium_count}
                    </span>
                  </td>
                  <td className="px-3 py-4 text-center">
                    <span
                      className={`text-sm font-semibold ${
                        review.low_count > 0
                          ? 'text-emerald-400'
                          : 'text-[#475569]'
                      }`}
                    >
                      {review.low_count}
                    </span>
                  </td>
                  <td className="px-4 py-4 text-right">
                    <span className="text-xs font-medium text-[#94a3b8] tabular-nums">
                      {review.cost_usd > 0 ? `$${review.cost_usd.toFixed(4)}` : '—'}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-right">
                    <span className="text-xs text-[#64748b]">
                      {formatDate(review.created_at)}
                    </span>
                  </td>
                </tr>
              ))}
              {paginatedReviews.length === 0 && (
                <tr>
                  <td
                    colSpan={9}
                    className="px-6 py-12 text-center text-sm text-[#64748b]"
                  >
                    No reviews match the selected filters.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="px-6 py-4 border-t border-[rgba(51,65,85,0.3)] flex items-center justify-between">
            <span className="text-xs text-[#64748b]">
              Page {currentPage} of {totalPages}
            </span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                disabled={currentPage === 1}
                className="
                  p-2 rounded-lg text-[#94a3b8]
                  hover:bg-[#1e293b] hover:text-[#f8fafc]
                  disabled:opacity-30 disabled:cursor-not-allowed
                  transition-all duration-200
                "
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              {Array.from({ length: totalPages }, (_, i) => i + 1).map(
                (page) => (
                  <button
                    key={page}
                    onClick={() => setCurrentPage(page)}
                    className={`
                      w-8 h-8 rounded-lg text-sm font-medium
                      transition-all duration-200
                      ${
                        currentPage === page
                          ? 'bg-[#3b82f6] text-white shadow-lg shadow-blue-500/25'
                          : 'text-[#94a3b8] hover:bg-[#1e293b] hover:text-[#f8fafc]'
                      }
                    `}
                  >
                    {page}
                  </button>
                )
              )}
              <button
                onClick={() =>
                  setCurrentPage((p) => Math.min(totalPages, p + 1))
                }
                disabled={currentPage === totalPages}
                className="
                  p-2 rounded-lg text-[#94a3b8]
                  hover:bg-[#1e293b] hover:text-[#f8fafc]
                  disabled:opacity-30 disabled:cursor-not-allowed
                  transition-all duration-200
                "
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
