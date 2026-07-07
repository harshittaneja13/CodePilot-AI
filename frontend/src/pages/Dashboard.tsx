import {
  GitFork,
  GitPullRequest,
  MessageSquareCode,
  AlertTriangle,
  DollarSign,
} from 'lucide-react';
import StatsCard from '@/components/Dashboard/StatsCard';
import ReviewChart from '@/components/Dashboard/ReviewChart';
import SeverityDonut from '@/components/Dashboard/SeverityDonut';
import RecentReviews from '@/components/Dashboard/RecentReviews';
import ActivityFeed from '@/components/Dashboard/ActivityFeed';
import LoadingSkeleton from '@/components/common/LoadingSkeleton';
import { useApi } from '@/hooks/useApi';
import { getDashboardStats } from '@/lib/api';
import { mockDashboardStats } from '@/lib/mockData';

export default function Dashboard() {
  const { data: stats, loading } = useApi(getDashboardStats, mockDashboardStats);

  if (loading || !stats) {
    return (
      <div className="space-y-8 max-w-7xl">
        <LoadingSkeleton type="card" count={4} />
      </div>
    );
  }

  return (
    <div className="space-y-8 max-w-7xl">
      {/* Page Header */}
      <div className="opacity-0 animate-fadeIn">
        <h1 className="text-2xl font-bold text-[#f8fafc]">Dashboard</h1>
        <p className="text-[#94a3b8] text-sm mt-1">
          Overview of your AI-powered code review activity
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-5">
        <StatsCard
          title="Repositories"
          value={stats.total_repositories}
          icon={GitFork}
          color="blue"
          delay={0}
        />
        <StatsCard
          title="Pull Requests"
          value={stats.total_pull_requests}
          icon={GitPullRequest}
          color="emerald"
          delay={100}
        />
        <StatsCard
          title="Reviews"
          value={stats.total_reviews}
          icon={MessageSquareCode}
          color="purple"
          delay={200}
        />
        <StatsCard
          title="Issues Found"
          value={stats.total_findings}
          icon={AlertTriangle}
          color="amber"
          delay={300}
        />
        <StatsCard
          title="LLM Cost"
          value={stats.total_cost_usd}
          icon={DollarSign}
          color="green"
          delay={400}
          format={(v) => `$${v.toFixed(2)}`}
        />
      </div>

      {/* Main content + live activity sidebar */}
      <div className="grid grid-cols-1 xl:grid-cols-4 gap-6">
        <div className="xl:col-span-3 space-y-8">
          {/* Charts Row */}
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
            <div className="lg:col-span-2">
              <ReviewChart activity={stats.review_activity} />
            </div>
            <SeverityDonut stats={stats} />
          </div>

          {/* Recent Reviews Table */}
          <RecentReviews reviews={stats.recent_reviews} />
        </div>

        {/* Live Activity sidebar */}
        <div className="xl:col-span-1">
          <ActivityFeed />
        </div>
      </div>
    </div>
  );
}
