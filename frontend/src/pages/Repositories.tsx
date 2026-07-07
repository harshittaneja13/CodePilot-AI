import { useState } from 'react';
import {
  GitFork,
  Plus,
  Settings,
  Circle,
  X,
  Loader2,
  Trash2,
} from 'lucide-react';
import { useApi } from '@/hooks/useApi';
import { getRepositories, createRepository, updateRepositorySettings, deleteRepository, getAvailableModels } from '@/lib/api';
import { mockRepositories } from '@/lib/mockData';
import type { Repository } from '@/lib/types';

const languageColors: Record<string, string> = {
  TypeScript: '#3178c6',
  Go: '#00add8',
  Swift: '#f05138',
  HCL: '#7b42bc',
  Python: '#3572a5',
  Rust: '#dea584',
  Java: '#b07219',
};

const FALLBACK_MODELS = [
  'llama-3.3-70b-versatile',
  'llama-3.1-8b-instant',
  'mixtral-8x7b-32768',
  'gemma2-9b-it',
];

function RepositoryCard({
  repo,
  onSettingsUpdated,
  onDeleted,
  availableModels,
}: {
  repo: Repository;
  onSettingsUpdated: () => void;
  onDeleted: () => void;
  availableModels: string[];
}) {
  const [showSettings, setShowSettings] = useState(false);
  const [autoReview, setAutoReview] = useState(repo.settings.auto_review);
  const [model, setModel] = useState(repo.settings.llm_model || availableModels[0] || FALLBACK_MODELS[0]);
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function saveSettings() {
    setSaving(true);
    try {
      await updateRepositorySettings(repo.id, { llm_model: model, auto_review: autoReview });
      onSettingsUpdated();
    } catch {
      // settings saved optimistically; silent failure is acceptable here
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    setDeleting(true);
    try {
      await deleteRepository(repo.id);
      onDeleted();
    } catch {
      setDeleting(false);
      setConfirmDelete(false);
    }
  }

  return (
    <div className="glass-card overflow-hidden hover:shadow-xl hover:shadow-blue-500/5 transition-all duration-300 group">
      <div className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-[#1e293b] border border-[#334155] flex items-center justify-center group-hover:border-[#3b82f6]/30 transition-colors">
              <GitFork className="w-5 h-5 text-[#94a3b8]" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-[#f8fafc] group-hover:text-[#60a5fa] transition-colors">
                {repo.full_name}
              </h3>
              <div className="flex items-center gap-2 mt-1">
                <div className="flex items-center gap-1.5">
                  <span
                    className="w-2.5 h-2.5 rounded-full"
                    style={{ backgroundColor: languageColors[repo.language] || '#64748b' }}
                  />
                  <span className="text-xs text-[#94a3b8]">{repo.language}</span>
                </div>
                <span className="text-[#334155]">·</span>
                <div className="flex items-center gap-1">
                  <Circle
                    className={`w-2 h-2 ${
                      repo.is_active
                        ? 'text-emerald-400 fill-emerald-400'
                        : 'text-[#64748b] fill-[#64748b]'
                    }`}
                  />
                  <span className="text-xs text-[#64748b]">
                    {repo.is_active ? 'Active' : 'Inactive'}
                  </span>
                </div>
              </div>
            </div>
          </div>
          <button
            onClick={() => setShowSettings(!showSettings)}
            className={`
              p-2 rounded-lg transition-all duration-200
              ${
                showSettings
                  ? 'bg-[#3b82f6]/10 text-[#3b82f6]'
                  : 'text-[#64748b] hover:bg-[#1e293b] hover:text-[#94a3b8]'
              }
            `}
          >
            <Settings className="w-4 h-4" />
          </button>
        </div>

        <div className="mt-4 flex items-center gap-4 text-xs text-[#64748b]">
          <span>
            Branch: <span className="text-[#94a3b8] font-mono">{repo.default_branch}</span>
          </span>
          <span>
            Max files: <span className="text-[#94a3b8]">{repo.settings.max_files_per_review}</span>
          </span>
        </div>
      </div>

      {showSettings && (
        <div className="border-t border-[rgba(51,65,85,0.5)] p-5 bg-[rgba(15,23,42,0.5)] animate-fadeIn">
          <h4 className="text-xs font-semibold text-[#94a3b8] uppercase tracking-wider mb-4">
            Review Settings
          </h4>
          <div className="space-y-4">
            <div>
              <label className="text-xs text-[#64748b] mb-1.5 block">LLM Model</label>
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="
                  w-full bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2
                  text-sm text-[#f8fafc]
                  focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30
                  transition-all
                "
              >
                {(availableModels.length > 0 ? availableModels : FALLBACK_MODELS).map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
            </div>

            <div className="flex items-center justify-between">
              <div>
                <span className="text-sm text-[#e2e8f0]">Auto Review</span>
                <p className="text-xs text-[#64748b] mt-0.5">Automatically review new PRs</p>
              </div>
              <button
                onClick={() => setAutoReview(!autoReview)}
                className={`
                  relative w-11 h-6 rounded-full transition-colors duration-200
                  ${autoReview ? 'bg-[#3b82f6]' : 'bg-[#334155]'}
                `}
              >
                <span
                  className={`
                    absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow-md
                    transition-transform duration-200
                    ${autoReview ? 'translate-x-5' : 'translate-x-0'}
                  `}
                />
              </button>
            </div>

            <div>
              <label className="text-xs text-[#64748b] mb-1.5 block">Exclude Patterns</label>
              <div className="flex flex-wrap gap-1.5">
                {(repo.settings.exclude_patterns || []).map((pattern) => (
                  <span
                    key={pattern}
                    className="inline-flex items-center gap-1 px-2 py-0.5 bg-[#1e293b] border border-[#334155] rounded text-xs text-[#94a3b8] font-mono"
                  >
                    {pattern}
                  </span>
                ))}
              </div>
            </div>

            <button
              onClick={saveSettings}
              disabled={saving}
              className="
                w-full py-2 rounded-lg text-sm font-medium
                bg-[#3b82f6] text-white
                hover:bg-[#2563eb] disabled:opacity-50
                transition-all duration-200
                flex items-center justify-center gap-2
              "
            >
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}
              {saving ? 'Saving…' : 'Save Settings'}
            </button>

            <div className="border-t border-[rgba(51,65,85,0.5)] pt-4">
              {confirmDelete ? (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-[#94a3b8] flex-1">Remove this repository?</span>
                  <button
                    onClick={() => setConfirmDelete(false)}
                    className="px-3 py-1.5 rounded-lg text-xs text-[#94a3b8] hover:bg-[#1e293b] transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleDelete}
                    disabled={deleting}
                    className="px-3 py-1.5 rounded-lg text-xs font-medium bg-red-500/10 text-red-400 hover:bg-red-500/20 disabled:opacity-50 transition-colors flex items-center gap-1"
                  >
                    {deleting && <Loader2 className="w-3 h-3 animate-spin" />}
                    {deleting ? 'Removing…' : 'Confirm Remove'}
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => setConfirmDelete(true)}
                  className="flex items-center gap-2 text-xs text-[#64748b] hover:text-red-400 transition-colors"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  Remove repository
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default function Repositories() {
  const { data: repos, loading, refetch } = useApi(getRepositories, mockRepositories);
  const { data: availableModels } = useApi(getAvailableModels, FALLBACK_MODELS);
  const [showForm, setShowForm] = useState(false);
  const [newOwner, setNewOwner] = useState('');
  const [newName, setNewName] = useState('');
  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState('');

  async function handleConnect() {
    if (!newOwner.trim() || !newName.trim()) return;
    setConnecting(true);
    setConnectError('');
    try {
      await createRepository({ owner: newOwner.trim(), name: newName.trim() });
      setNewOwner('');
      setNewName('');
      setShowForm(false);
      refetch();
    } catch (err) {
      setConnectError(err instanceof Error ? err.message : 'Failed to connect repository');
    } finally {
      setConnecting(false);
    }
  }

  return (
    <div className="space-y-6 max-w-5xl animate-fadeIn">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-[#f8fafc]">Repositories</h1>
          <p className="text-[#94a3b8] text-sm mt-1">
            Manage connected repositories and review settings
          </p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="
            flex items-center gap-2 px-4 py-2.5 rounded-lg font-medium text-sm
            bg-[#3b82f6] text-white
            hover:bg-[#2563eb] hover:scale-105 hover:shadow-lg hover:shadow-blue-500/25
            active:scale-95 transition-all duration-200
          "
        >
          <Plus className="w-4 h-4" />
          Connect Repository
        </button>
      </div>

      {showForm && (
        <div className="glass-card p-5 animate-slideUp">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-semibold text-[#f8fafc]">Connect a Repository</h3>
            <button
              onClick={() => { setShowForm(false); setConnectError(''); }}
              className="p-1 rounded hover:bg-[#1e293b] transition-colors"
            >
              <X className="w-4 h-4 text-[#64748b]" />
            </button>
          </div>
          {connectError && (
            <p className="text-xs text-red-400 mb-3">{connectError}</p>
          )}
          <div className="flex gap-3">
            <input
              type="text"
              placeholder="Owner (e.g. acme)"
              value={newOwner}
              onChange={(e) => setNewOwner(e.target.value)}
              className="
                flex-1 bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2.5
                text-sm text-[#f8fafc] placeholder-[#475569]
                focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30
                transition-all
              "
            />
            <input
              type="text"
              placeholder="Repository name"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleConnect()}
              className="
                flex-1 bg-[#0f172a] border border-[#334155] rounded-lg px-3 py-2.5
                text-sm text-[#f8fafc] placeholder-[#475569]
                focus:outline-none focus:border-[#3b82f6] focus:ring-1 focus:ring-[#3b82f6]/30
                transition-all
              "
            />
            <button
              onClick={handleConnect}
              disabled={connecting || !newOwner.trim() || !newName.trim()}
              className="
                px-5 py-2.5 rounded-lg font-medium text-sm
                bg-[#3b82f6] text-white
                hover:bg-[#2563eb] disabled:opacity-50
                active:scale-95 transition-all duration-200
                flex items-center gap-2
              "
            >
              {connecting && <Loader2 className="w-4 h-4 animate-spin" />}
              {connecting ? 'Connecting…' : 'Connect'}
            </button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="glass-card p-5 h-36 animate-pulse bg-[#1e293b]/50" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {(repos || []).map((repo) => (
            <RepositoryCard key={repo.id} repo={repo} onSettingsUpdated={refetch} onDeleted={refetch} availableModels={availableModels || FALLBACK_MODELS} />
          ))}
        </div>
      )}
    </div>
  );
}
