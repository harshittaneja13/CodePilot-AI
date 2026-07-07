// ─── Repository ─────────────────────────────────────────────────────────────

export interface RepositorySettings {
  llm_model: string;
  auto_review: boolean;
  review_on_draft?: boolean;
  exclude_patterns: string[];
  max_files_per_review: number;
}

export interface Repository {
  id: string;
  github_id: number;
  owner: string;
  name: string;
  full_name: string;
  language: string;
  default_branch: string;
  is_active: boolean;
  webhook_id: number | null;
  settings: RepositorySettings;
  created_at: string;
  updated_at: string;
}

// ─── Pull Request ───────────────────────────────────────────────────────────

export interface PullRequest {
  id: string;
  repository_id: string;
  github_number: number;
  title: string;
  body: string;
  state: 'open' | 'closed' | 'merged';
  author: string;
  head_branch: string;
  base_branch: string;
  head_sha: string;
  additions: number;
  deletions: number;
  changed_files: number;
  github_url: string;
  created_at: string;
  updated_at: string;
}

// ─── Review ─────────────────────────────────────────────────────────────────

export type ReviewStatus = 'pending' | 'in_progress' | 'completed' | 'failed';

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info';

export interface ReviewComment {
  id: string;
  review_id: string;
  file_path: string;
  line_number: number | null;
  severity: Severity;
  title: string;
  explanation: string;
  why_it_matters: string;
  suggestion: string;
  code_snippet: string;
  published: boolean;
  created_at: string;
}

export interface Review {
  id: string;
  pull_request_id: string;
  status: ReviewStatus;
  summary: string;
  total_comments: number;
  critical_count: number;
  high_count: number;
  medium_count: number;
  low_count: number;
  llm_model: string;
  tokens_used: number;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
  processing_time_ms: number;
  error_message: string;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

export interface ReviewWithComments {
  review: Review;
  comments: ReviewComment[];
}

// ─── Execution Log (agent trace / pipeline step) ────────────────────────────

export interface ExecutionLog {
  id: string;
  review_id: string;
  step: string;
  status: string;
  message?: string;
  duration_ms: number;
  created_at: string;
}

// ─── Activity Event (pipeline milestone for the live feed) ──────────────────

export interface ActivityEvent {
  id: string;
  review_id: string;
  pull_request_id: string;
  repository_name: string;
  pr_number: number;
  pr_title: string;
  step: string;
  status: string;
  message?: string;
  duration_ms: number;
  created_at: string;
}

// ─── Dashboard Review (enriched with repo + PR context) ─────────────────────

export interface DashboardReview {
  id: string;
  pull_request_id: string;
  repository_id: string;
  repository_name: string;
  pr_number: number;
  pr_title: string;
  status: ReviewStatus;
  llm_model: string;
  total_findings: number;
  critical_count: number;
  high_count: number;
  medium_count: number;
  low_count: number;
  tokens_used: number;
  cost_usd: number;
  processing_time_ms: number;
  error_message: string;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

// ─── Changed File ───────────────────────────────────────────────────────────

export interface ChangedFile {
  filename: string;
  status: 'added' | 'modified' | 'deleted' | 'renamed';
  additions: number;
  deletions: number;
  patch: string;
}

// ─── Dashboard ──────────────────────────────────────────────────────────────

export interface ReviewActivity {
  date: string;
  reviews: number;
  findings: number;
}

export interface DashboardStats {
  total_repositories: number;
  total_pull_requests: number;
  total_reviews: number;
  total_findings: number;
  critical_findings: number;
  high_findings: number;
  medium_findings: number;
  low_findings: number;
  reviews_today: number;
  reviews_this_week: number;
  total_cost_usd: number;
  average_review_time_ms: number;
  review_activity: ReviewActivity[];
  recent_reviews: DashboardReview[];
}

// ─── API Params ─────────────────────────────────────────────────────────────

export interface PaginationParams {
  limit?: number;
  offset?: number;
}

export interface ReviewFilterParams extends PaginationParams {
  repo_id?: string;
  status?: ReviewStatus;
}

export interface PullRequestFilterParams extends PaginationParams {
  repo_id?: string;
  state?: string;
}
