import { useNavigate } from 'react-router-dom';
import type { DashboardReview } from '@/lib/types';
import Badge from '@/components/common/Badge';
import { formatTimeAgo } from '@/lib/format';
import { Clock, ExternalLink } from 'lucide-react';

function formatStatus(status: string): string {
  return status.replace('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

interface Props {
  reviews: DashboardReview[];
}

export default function RecentReviews({ reviews }: Props) {
  const navigate = useNavigate();

  return (
    <div className="glass-card overflow-hidden animate-fadeIn" style={{ animationDelay: '300ms' }}>
      <div className="px-6 py-4 border-b border-[rgba(51,65,85,0.5)] flex items-center justify-between">
        <h3 className="text-base font-semibold text-[#f8fafc]">Recent Reviews</h3>
        <button
          onClick={() => navigate('/reviews')}
          className="text-xs text-[#3b82f6] hover:text-[#60a5fa] font-medium flex items-center gap-1 transition-colors"
        >
          View all <ExternalLink className="w-3 h-3" />
        </button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-[rgba(51,65,85,0.3)]">
              <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                Repository
              </th>
              <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                Pull Request
              </th>
              <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                Status
              </th>
              <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                Findings
              </th>
              <th className="text-right text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                Time
              </th>
            </tr>
          </thead>
          <tbody>
            {reviews.map((review) => (
              <tr
                key={review.id}
                onClick={() => navigate(`/pull-requests/${review.pull_request_id}`)}
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
                    <span className="text-sm text-[#f8fafc] font-medium truncate max-w-[260px]">
                      {review.pr_title}
                    </span>
                    <span className="text-xs text-[#64748b] font-mono">
                      #{review.pr_number}
                    </span>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <Badge variant={review.status}>{formatStatus(review.status)}</Badge>
                </td>
                <td className="px-6 py-4">
                  <div className="flex items-center gap-1.5">
                    <span className="text-sm font-semibold text-[#f8fafc]">
                      {review.total_findings}
                    </span>
                    {review.critical_count > 0 && (
                      <span className="w-5 h-5 rounded text-[10px] font-bold flex items-center justify-center bg-red-500/20 text-red-400">
                        {review.critical_count}
                      </span>
                    )}
                    {review.high_count > 0 && (
                      <span className="w-5 h-5 rounded text-[10px] font-bold flex items-center justify-center bg-amber-500/20 text-amber-400">
                        {review.high_count}
                      </span>
                    )}
                    {review.medium_count > 0 && (
                      <span className="w-5 h-5 rounded text-[10px] font-bold flex items-center justify-center bg-blue-500/20 text-blue-400">
                        {review.medium_count}
                      </span>
                    )}
                  </div>
                </td>
                <td className="px-6 py-4 text-right">
                  <span className="text-xs text-[#64748b] flex items-center justify-end gap-1">
                    <Clock className="w-3 h-3" />
                    {formatTimeAgo(review.created_at)}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
