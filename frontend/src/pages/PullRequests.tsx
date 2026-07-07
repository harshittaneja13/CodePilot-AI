import { useState, useMemo, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  GitPullRequest,
  Plus,
  Minus,
  Files,
  ChevronLeft,
  ChevronRight,
  SlidersHorizontal,
  ArrowUpDown,
  GitFork,
  RefreshCw,
} from 'lucide-react';
import Badge from '@/components/common/Badge';
import { useApi } from '@/hooks/useApi';
import { getPullRequests, getRepositories, syncRepository } from '@/lib/api';
import { mockPullRequests, mockRepositories } from '@/lib/mockData';
import type { PullRequest, Repository } from '@/lib/types';

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

const PAGE_SIZE = 15;

function PRRow({ pr }: { pr: PullRequest }) {
  const navigate = useNavigate();

  return (
    <tr
      onClick={() => navigate(`/pull-requests/${pr.id}`)}
      className="border-b border-[rgba(51,65,85,0.2)] cursor-pointer hover:bg-[rgba(59,130,246,0.05)] transition-colors duration-150"
    >
      <td className="px-6 py-4">
        <div className="flex items-start gap-3">
          <GitPullRequest className="w-4 h-4 text-[#64748b] mt-0.5 shrink-0" />
          <div className="min-w-0">
            <span className="text-sm font-medium text-[#f8fafc] block truncate max-w-[320px]">
              {pr.title}
            </span>
            <div className="flex items-center gap-2 mt-1 text-xs text-[#64748b]">
              <span className="font-mono">#{pr.github_number}</span>
              <span>·</span>
              <span>{pr.author}</span>
              <span>·</span>
              <span className="font-mono text-[#94a3b8]">{pr.head_branch}</span>
            </div>
          </div>
        </div>
      </td>
      <td className="px-6 py-4">
        <Badge variant={pr.state}>{pr.state}</Badge>
      </td>
      <td className="px-6 py-4">
        <div className="flex items-center gap-3 text-xs">
          <span className="text-emerald-400 font-mono flex items-center gap-1">
            <Plus className="w-3 h-3" />
            {pr.additions.toLocaleString()}
          </span>
          <span className="text-red-400 font-mono flex items-center gap-1">
            <Minus className="w-3 h-3" />
            {pr.deletions.toLocaleString()}
          </span>
          <span className="text-[#94a3b8] flex items-center gap-1">
            <Files className="w-3 h-3" />
            {pr.changed_files}
          </span>
        </div>
      </td>
      <td className="px-6 py-4 text-right">
        <span className="text-xs text-[#64748b]">{formatDate(pr.created_at)}</span>
      </td>
    </tr>
  );
}

export default function PullRequests() {
  const { data: repos } = useApi(getRepositories, mockRepositories);

  const [selectedRepoId, setSelectedRepoId] = useState<string>('');
  const [stateFilter, setStateFilter] = useState<string>('all');
  const [sortAsc, setSortAsc] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [syncing, setSyncing] = useState(false);
  const [syncMsg, setSyncMsg] = useState('');

  // Auto-select the first repo once the list loads.
  useEffect(() => {
    if (repos && repos.length > 0 && !selectedRepoId) {
      setSelectedRepoId(repos[0].id);
    }
  }, [repos, selectedRepoId]);

  const { data: prs, loading, refetch: refetchPRs } = useApi(
    () => getPullRequests(selectedRepoId ? { repo_id: selectedRepoId } : undefined),
    mockPullRequests,
    [selectedRepoId],
  );

  async function handleSync() {
    if (!selectedRepoId || syncing) return;
    setSyncing(true);
    setSyncMsg('');
    try {
      const result = await syncRepository(selectedRepoId);
      setSyncMsg(`Synced ${result.synced} PR${result.synced !== 1 ? 's' : ''}`);
      await refetchPRs();
    } catch {
      setSyncMsg('Sync failed');
    } finally {
      setSyncing(false);
      setTimeout(() => setSyncMsg(''), 4000);
    }
  }

  const filtered = useMemo(() => {
    let result = [...(prs ?? [])];
    if (stateFilter !== 'all') result = result.filter((p) => p.state === stateFilter);
    result.sort((a, b) => {
      const diff = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
      return sortAsc ? diff : -diff;
    });
    return result;
  }, [prs, stateFilter, sortAsc]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const paginated = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE);

  const selectedRepo = repos?.find((r: Repository) => r.id === selectedRepoId);

  function selectRepo(id: string) {
    setSelectedRepoId(id);
    setCurrentPage(1);
    setStateFilter('all');
  }

  return (
    <div className="space-y-6 max-w-6xl animate-fadeIn">
      <div>
        <h1 className="text-2xl font-bold text-[#f8fafc]">Pull Requests</h1>
        <p className="text-[#94a3b8] text-sm mt-1">
          {selectedRepo ? selectedRepo.full_name : 'Select a repository to view pull requests'}
        </p>
      </div>

      {/* Repo tabs */}
      {repos && repos.length > 0 && (
        <div className="flex items-center gap-2 flex-wrap">
          {repos.map((repo: Repository) => (
            <button
              key={repo.id}
              onClick={() => selectRepo(repo.id)}
              className={`
                flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200
                ${selectedRepoId === repo.id
                  ? 'bg-[#3b82f6] text-white shadow-lg shadow-blue-500/20'
                  : 'bg-[#1e293b] text-[#94a3b8] hover:text-[#f8fafc] hover:bg-[#263248] border border-[#334155]'
                }
              `}
            >
              <GitFork className="w-3.5 h-3.5" />
              {repo.name}
            </button>
          ))}
        </div>
      )}

      {/* Filter bar */}
      <div className="glass-card px-5 py-4 flex items-center gap-4 flex-wrap">
        <SlidersHorizontal className="w-4 h-4 text-[#64748b]" />
        <select
          value={stateFilter}
          onChange={(e) => { setStateFilter(e.target.value); setCurrentPage(1); }}
          className="
            bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2
            text-sm text-[#f8fafc]
            focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30
          "
        >
          <option value="all">All States</option>
          <option value="open">Open</option>
          <option value="closed">Closed</option>
          <option value="merged">Merged</option>
        </select>
        <button
          onClick={() => setSortAsc(!sortAsc)}
          className="
            flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm
            text-[#94a3b8] hover:text-[#f8fafc] hover:bg-[#1e293b]
            border border-[#334155] transition-all duration-200
          "
        >
          <ArrowUpDown className="w-3.5 h-3.5" />
          {sortAsc ? 'Oldest First' : 'Newest First'}
        </button>
        <div className="ml-auto flex items-center gap-3">
          {syncMsg && (
            <span className="text-xs text-emerald-400">{syncMsg}</span>
          )}
          {selectedRepoId && (
            <button
              onClick={handleSync}
              disabled={syncing}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm text-[#94a3b8] hover:text-[#f8fafc] hover:bg-[#1e293b] border border-[#334155] transition-all duration-200 disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${syncing ? 'animate-spin' : ''}`} />
              {syncing ? 'Syncing…' : 'Sync PRs'}
            </button>
          )}
          <span className="text-xs text-[#64748b]">
            {filtered.length} pull request{filtered.length !== 1 ? 's' : ''}
          </span>
        </div>
      </div>

      <div className="glass-card overflow-hidden">
        {loading ? (
          <div className="p-12 text-center text-[#64748b] text-sm">Loading…</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-[rgba(51,65,85,0.5)]">
                  <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                    Pull Request
                  </th>
                  <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                    State
                  </th>
                  <th className="text-left text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                    Changes
                  </th>
                  <th className="text-right text-xs font-medium text-[#64748b] uppercase tracking-wider px-6 py-3">
                    Created
                  </th>
                </tr>
              </thead>
              <tbody>
                {paginated.map((pr) => (
                  <PRRow key={pr.id} pr={pr} />
                ))}
                {paginated.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-6 py-12 text-center text-sm text-[#64748b]">
                      {selectedRepoId
                        ? 'No pull requests match the selected filter.'
                        : 'Select a repository above to view its pull requests.'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}

        {totalPages > 1 && (
          <div className="px-6 py-4 border-t border-[rgba(51,65,85,0.3)] flex items-center justify-between">
            <span className="text-xs text-[#64748b]">Page {currentPage} of {totalPages}</span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                disabled={currentPage === 1}
                className="p-2 rounded-lg text-[#94a3b8] hover:bg-[#1e293b] disabled:opacity-30 transition-all"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage === totalPages}
                className="p-2 rounded-lg text-[#94a3b8] hover:bg-[#1e293b] disabled:opacity-30 transition-all"
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
