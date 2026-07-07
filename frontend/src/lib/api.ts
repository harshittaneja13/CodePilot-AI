import axios from 'axios';
import type {
  DashboardStats,
  Repository,
  RepositorySettings,
  PullRequest,
  Review,
  ReviewWithComments,
  ReviewFilterParams,
  PullRequestFilterParams,
  ChangedFile,
  ExecutionLog,
  ActivityEvent,
} from './types';
import {
  mockDashboardStats,
  mockRepositories,
  mockPullRequests,
  mockReviewWithComments,
  mockActivity,
  mockChangedFiles,
  mockExecutionLogs,
} from './mockData';

// ─── Demo mode ──────────────────────────────────────────────────────────────
// When VITE_DEMO_MODE=true (set for the static Vercel demo), the API layer serves
// built-in mock data instead of hitting a backend — so the demo is fully populated,
// makes no network calls, and never logs errors. Reads return mock data; mutations no-op.

const DEMO = import.meta.env.VITE_DEMO_MODE === 'true';

/** Resolve mock data after a short delay so the demo feels like a real fetch. */
function demoResolve<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), 150));
}

const DEMO_MODELS = ['llama-3.3-70b-versatile', 'gpt-4o', 'gpt-4o-mini', 'claude-3-5-sonnet'];

// ─── Axios Instance ─────────────────────────────────────────────────────────

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 15000,
});

// Add response interceptor for consistent error handling
api.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('[API Error]', error?.response?.data || error.message);
    return Promise.reject(error);
  }
);

// ─── Dashboard ──────────────────────────────────────────────────────────────

export async function getDashboardStats(): Promise<DashboardStats> {
  if (DEMO) return demoResolve(mockDashboardStats);
  const { data } = await api.get<DashboardStats>('/dashboard/stats');
  return data;
}

export async function getActivity(limit = 30): Promise<ActivityEvent[]> {
  if (DEMO) return demoResolve(mockActivity);
  const { data } = await api.get<ActivityEvent[]>('/dashboard/activity', {
    params: { limit },
  });
  return data;
}

// ─── Repositories ───────────────────────────────────────────────────────────

export async function getRepositories(): Promise<Repository[]> {
  if (DEMO) return demoResolve(mockRepositories);
  const { data } = await api.get<Repository[]>('/repositories');
  return data;
}

export async function getRepositoryById(id: string): Promise<Repository> {
  if (DEMO) return demoResolve(mockRepositories.find((r) => r.id === id) ?? mockRepositories[0]);
  const { data } = await api.get<Repository>(`/repositories/${id}`);
  return data;
}

export async function createRepository(payload: {
  owner: string;
  name: string;
}): Promise<Repository> {
  if (DEMO) return demoResolve({ ...mockRepositories[0], ...payload, full_name: `${payload.owner}/${payload.name}` });
  const { data } = await api.post<Repository>('/repositories', payload);
  return data;
}

export async function updateRepositorySettings(
  id: string,
  settings: Partial<RepositorySettings>
): Promise<void> {
  if (DEMO) return demoResolve(undefined);
  await api.patch(`/repositories/${id}/settings`, settings);
}

export async function deleteRepository(id: string): Promise<void> {
  if (DEMO) return demoResolve(undefined);
  await api.delete(`/repositories/${id}`);
}

export async function syncRepository(id: string): Promise<{ synced: number }> {
  if (DEMO) return demoResolve({ synced: 3 });
  const { data } = await api.post(`/repositories/${id}/sync`);
  return data;
}

// ─── Pull Requests ──────────────────────────────────────────────────────────

export async function getPullRequests(
  params?: PullRequestFilterParams
): Promise<PullRequest[]> {
  if (DEMO) return demoResolve(mockPullRequests);
  const { data } = await api.get<PullRequest[]>('/pull-requests', { params });
  return data;
}

export async function getPullRequestById(id: string): Promise<{
  pull_request: PullRequest;
  reviews: Review[];
}> {
  if (DEMO) {
    const pr = mockPullRequests.find((p) => p.id === id) ?? mockPullRequests[0];
    return demoResolve({ pull_request: pr, reviews: [mockReviewWithComments.review] });
  }
  const { data } = await api.get(`/pull-requests/${id}`);
  return data;
}

export async function getPullRequestFiles(id: string): Promise<ChangedFile[]> {
  if (DEMO) return demoResolve(mockChangedFiles);
  const { data } = await api.get(`/pull-requests/${id}/files`);
  return data;
}

export async function triggerPullRequestReview(id: string, model?: string): Promise<void> {
  if (DEMO) return demoResolve(undefined);
  await api.post(`/pull-requests/${id}/trigger-review`, model ? { model } : {});
}

export async function getAvailableModels(): Promise<string[]> {
  if (DEMO) return demoResolve(DEMO_MODELS);
  const { data } = await api.get<string[]>('/models');
  return data;
}

// ─── Reviews ────────────────────────────────────────────────────────────────

export async function getReviews(
  params?: ReviewFilterParams
): Promise<Review[]> {
  if (DEMO) return demoResolve([mockReviewWithComments.review]);
  const { data } = await api.get<Review[]>('/reviews', { params });
  return data;
}

export async function getReviewById(id: string): Promise<ReviewWithComments> {
  if (DEMO) return demoResolve(mockReviewWithComments);
  const { data } = await api.get<ReviewWithComments>(`/reviews/${id}`);
  return data;
}

export async function getReviewLogs(id: string): Promise<ExecutionLog[]> {
  if (DEMO) return demoResolve(mockExecutionLogs);
  const { data } = await api.get<ExecutionLog[]>(`/reviews/${id}/logs`);
  return data;
}

export async function retryReview(id: string): Promise<void> {
  if (DEMO) return demoResolve(undefined);
  await api.post(`/reviews/${id}/retry`);
}

export default api;
