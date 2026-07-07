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
  const { data } = await api.get<DashboardStats>('/dashboard/stats');
  return data;
}

export async function getActivity(limit = 30): Promise<ActivityEvent[]> {
  const { data } = await api.get<ActivityEvent[]>('/dashboard/activity', {
    params: { limit },
  });
  return data;
}

// ─── Repositories ───────────────────────────────────────────────────────────

export async function getRepositories(): Promise<Repository[]> {
  const { data } = await api.get<Repository[]>('/repositories');
  return data;
}

export async function getRepositoryById(id: string): Promise<Repository> {
  const { data } = await api.get<Repository>(`/repositories/${id}`);
  return data;
}

export async function createRepository(payload: {
  owner: string;
  name: string;
}): Promise<Repository> {
  const { data } = await api.post<Repository>('/repositories', payload);
  return data;
}

export async function updateRepositorySettings(
  id: string,
  settings: Partial<RepositorySettings>
): Promise<void> {
  await api.patch(`/repositories/${id}/settings`, settings);
}

export async function deleteRepository(id: string): Promise<void> {
  await api.delete(`/repositories/${id}`);
}

export async function syncRepository(id: string): Promise<{ synced: number }> {
  const { data } = await api.post(`/repositories/${id}/sync`);
  return data;
}

// ─── Pull Requests ──────────────────────────────────────────────────────────

export async function getPullRequests(
  params?: PullRequestFilterParams
): Promise<PullRequest[]> {
  const { data } = await api.get<PullRequest[]>('/pull-requests', { params });
  return data;
}

export async function getPullRequestById(id: string): Promise<{
  pull_request: PullRequest;
  reviews: Review[];
}> {
  const { data } = await api.get(`/pull-requests/${id}`);
  return data;
}

export async function getPullRequestFiles(id: string): Promise<ChangedFile[]> {
  const { data } = await api.get(`/pull-requests/${id}/files`);
  return data;
}

export async function triggerPullRequestReview(id: string, model?: string): Promise<void> {
  await api.post(`/pull-requests/${id}/trigger-review`, model ? { model } : {});
}

export async function getAvailableModels(): Promise<string[]> {
  const { data } = await api.get<string[]>('/models');
  return data;
}

// ─── Reviews ────────────────────────────────────────────────────────────────

export async function getReviews(
  params?: ReviewFilterParams
): Promise<Review[]> {
  const { data } = await api.get<Review[]>('/reviews', { params });
  return data;
}

export async function getReviewById(id: string): Promise<ReviewWithComments> {
  const { data } = await api.get<ReviewWithComments>(`/reviews/${id}`);
  return data;
}

export async function getReviewLogs(id: string): Promise<ExecutionLog[]> {
  const { data } = await api.get<ExecutionLog[]>(`/reviews/${id}/logs`);
  return data;
}

export async function retryReview(id: string): Promise<void> {
  await api.post(`/reviews/${id}/retry`);
}

export default api;
