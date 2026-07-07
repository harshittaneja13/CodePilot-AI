import type {
  DashboardStats,
  DashboardReview,
  ReviewActivity,
  Repository,
  PullRequest,
  ReviewWithComments,
  ReviewComment,
  ChangedFile,
  ActivityEvent,
  ExecutionLog,
} from './types';

// ─── Mock Review Activity ───────────────────────────────────────────────────

const mockReviewActivity: ReviewActivity[] = Array.from({ length: 14 }, (_, i) => {
  const d = new Date();
  d.setDate(d.getDate() - (13 - i));
  return {
    date: d.toISOString().split('T')[0],
    reviews: Math.floor(Math.random() * 12) + 2,
    findings: Math.floor(Math.random() * 35) + 5,
  };
});

// ─── Mock Recent Reviews (DashboardReview) ──────────────────────────────────

export const mockRecentReviews: DashboardReview[] = [
  {
    id: 'a1b2c3d4-0001-0001-0001-000000000001',
    pull_request_id: 'a1b2c3d4-0002-0001-0001-000000000001',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000001',
    repository_name: 'acme/web-app',
    pr_number: 342,
    pr_title: 'feat: Add user authentication flow',
    status: 'completed',
    llm_model: 'gpt-4o',
    total_findings: 8,
    critical_count: 1,
    high_count: 2,
    medium_count: 3,
    low_count: 2,
    tokens_used: 4200,
    cost_usd: 0.021,
    processing_time_ms: 72000,
    error_message: '',
    started_at: '2026-07-04T14:30:00Z',
    completed_at: '2026-07-04T14:31:12Z',
    created_at: '2026-07-04T14:30:00Z',
  },
  {
    id: 'a1b2c3d4-0001-0001-0001-000000000002',
    pull_request_id: 'a1b2c3d4-0002-0001-0001-000000000002',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000002',
    repository_name: 'acme/api-service',
    pr_number: 189,
    pr_title: 'fix: Resolve N+1 query in orders endpoint',
    status: 'completed',
    llm_model: 'gpt-4o',
    total_findings: 3,
    critical_count: 0,
    high_count: 1,
    medium_count: 1,
    low_count: 1,
    tokens_used: 1800,
    cost_usd: 0.009,
    processing_time_ms: 45000,
    error_message: '',
    started_at: '2026-07-04T12:15:00Z',
    completed_at: '2026-07-04T12:15:45Z',
    created_at: '2026-07-04T12:15:00Z',
  },
  {
    id: 'a1b2c3d4-0001-0001-0001-000000000003',
    pull_request_id: 'a1b2c3d4-0002-0001-0001-000000000003',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000001',
    repository_name: 'acme/web-app',
    pr_number: 341,
    pr_title: 'refactor: Migrate to React Server Components',
    status: 'in_progress',
    llm_model: 'gpt-4o',
    total_findings: 0,
    critical_count: 0,
    high_count: 0,
    medium_count: 0,
    low_count: 0,
    tokens_used: 0,
    cost_usd: 0,
    processing_time_ms: 0,
    error_message: '',
    started_at: '2026-07-05T02:50:00Z',
    completed_at: null,
    created_at: '2026-07-05T02:50:00Z',
  },
  {
    id: 'a1b2c3d4-0001-0001-0001-000000000004',
    pull_request_id: 'a1b2c3d4-0002-0001-0001-000000000004',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000003',
    repository_name: 'acme/mobile-sdk',
    pr_number: 56,
    pr_title: 'feat: Add biometric login support',
    status: 'completed',
    llm_model: 'claude-3.5-sonnet',
    total_findings: 12,
    critical_count: 2,
    high_count: 4,
    medium_count: 4,
    low_count: 2,
    tokens_used: 6100,
    cost_usd: 0.031,
    processing_time_ms: 150000,
    error_message: '',
    started_at: '2026-07-03T09:00:00Z',
    completed_at: '2026-07-03T09:02:30Z',
    created_at: '2026-07-03T09:00:00Z',
  },
  {
    id: 'a1b2c3d4-0001-0001-0001-000000000005',
    pull_request_id: 'a1b2c3d4-0002-0001-0001-000000000005',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000002',
    repository_name: 'acme/api-service',
    pr_number: 188,
    pr_title: 'chore: Update dependencies',
    status: 'failed',
    llm_model: 'gpt-4o',
    total_findings: 0,
    critical_count: 0,
    high_count: 0,
    medium_count: 0,
    low_count: 0,
    tokens_used: 0,
    cost_usd: 0,
    processing_time_ms: 10000,
    error_message: 'Rate limit exceeded',
    started_at: '2026-07-03T16:00:00Z',
    completed_at: '2026-07-03T16:00:10Z',
    created_at: '2026-07-03T16:00:00Z',
  },
  {
    id: 'a1b2c3d4-0001-0001-0001-000000000006',
    pull_request_id: 'a1b2c3d4-0002-0001-0001-000000000006',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000004',
    repository_name: 'acme/infra-tools',
    pr_number: 23,
    pr_title: 'feat: Terraform module for EKS cluster',
    status: 'completed',
    llm_model: 'gpt-4o',
    total_findings: 5,
    critical_count: 0,
    high_count: 1,
    medium_count: 3,
    low_count: 1,
    tokens_used: 2900,
    cost_usd: 0.015,
    processing_time_ms: 60000,
    error_message: '',
    started_at: '2026-07-02T11:30:00Z',
    completed_at: '2026-07-02T11:31:00Z',
    created_at: '2026-07-02T11:30:00Z',
  },
];

// ─── Mock Dashboard Stats ───────────────────────────────────────────────────

export const mockDashboardStats: DashboardStats = {
  total_repositories: 12,
  total_pull_requests: 247,
  total_reviews: 1_843,
  total_findings: 4_291,
  critical_findings: 89,
  high_findings: 342,
  medium_findings: 1_567,
  low_findings: 2_293,
  reviews_today: 14,
  reviews_this_week: 67,
  total_cost_usd: 12.47,
  average_review_time_ms: 68_500,
  review_activity: mockReviewActivity,
  recent_reviews: mockRecentReviews,
};

// ─── Mock Repositories ──────────────────────────────────────────────────────

export const mockRepositories: Repository[] = [
  {
    id: 'a1b2c3d4-0003-0001-0001-000000000001',
    github_id: 900001,
    owner: 'acme',
    name: 'web-app',
    full_name: 'acme/web-app',
    language: 'TypeScript',
    default_branch: 'main',
    is_active: true,
    webhook_id: 12345,
    settings: {
      llm_model: 'gpt-4o',
      auto_review: true,
      exclude_patterns: ['*.test.ts', '*.spec.ts', 'node_modules/**'],
      max_files_per_review: 20,
    },
    created_at: '2026-05-10T08:00:00Z',
    updated_at: '2026-07-04T14:30:00Z',
  },
  {
    id: 'a1b2c3d4-0003-0001-0001-000000000002',
    github_id: 900002,
    owner: 'acme',
    name: 'api-service',
    full_name: 'acme/api-service',
    language: 'Go',
    default_branch: 'main',
    is_active: true,
    webhook_id: 12346,
    settings: {
      llm_model: 'gpt-4o',
      auto_review: true,
      exclude_patterns: ['vendor/**', '*_test.go'],
      max_files_per_review: 15,
    },
    created_at: '2026-05-12T10:00:00Z',
    updated_at: '2026-07-04T12:15:00Z',
  },
  {
    id: 'a1b2c3d4-0003-0001-0001-000000000003',
    github_id: 900003,
    owner: 'acme',
    name: 'mobile-sdk',
    full_name: 'acme/mobile-sdk',
    language: 'Swift',
    default_branch: 'develop',
    is_active: true,
    webhook_id: null,
    settings: {
      llm_model: 'claude-3.5-sonnet',
      auto_review: false,
      exclude_patterns: ['Pods/**'],
      max_files_per_review: 10,
    },
    created_at: '2026-06-01T12:00:00Z',
    updated_at: '2026-07-03T09:00:00Z',
  },
  {
    id: 'a1b2c3d4-0003-0001-0001-000000000004',
    github_id: 900004,
    owner: 'acme',
    name: 'infra-tools',
    full_name: 'acme/infra-tools',
    language: 'HCL',
    default_branch: 'main',
    is_active: false,
    webhook_id: null,
    settings: {
      llm_model: 'gpt-4o',
      auto_review: true,
      exclude_patterns: ['.terraform/**'],
      max_files_per_review: 25,
    },
    created_at: '2026-06-15T14:00:00Z',
    updated_at: '2026-07-02T11:30:00Z',
  },
];

// ─── Mock Pull Requests ─────────────────────────────────────────────────────

export const mockPullRequests: PullRequest[] = [
  {
    id: 'a1b2c3d4-0002-0001-0001-000000000001',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000001',
    github_number: 342,
    title: 'feat: Add user authentication flow',
    body: 'Implements OAuth2 + PKCE authentication flow with JWT token refresh, login/signup pages, and protected route middleware.',
    state: 'open',
    author: 'sarahchen',
    head_branch: 'feat/auth-flow',
    base_branch: 'main',
    head_sha: 'abc123def456',
    additions: 1247,
    deletions: 89,
    changed_files: 18,
    github_url: 'https://github.com/acme/web-app/pull/342',
    created_at: '2026-07-04T14:00:00Z',
    updated_at: '2026-07-04T14:31:12Z',
  },
  {
    id: 'a1b2c3d4-0002-0001-0001-000000000002',
    repository_id: 'a1b2c3d4-0003-0001-0001-000000000002',
    github_number: 189,
    title: 'fix: Resolve N+1 query in orders endpoint',
    body: 'Adds eager loading for order items and customer data to eliminate N+1 queries.',
    state: 'merged',
    author: 'mike_dev',
    head_branch: 'fix/n-plus-one',
    base_branch: 'main',
    head_sha: 'def456abc789',
    additions: 342,
    deletions: 156,
    changed_files: 7,
    github_url: 'https://github.com/acme/api-service/pull/189',
    created_at: '2026-07-04T11:00:00Z',
    updated_at: '2026-07-04T12:15:45Z',
  },
];

// ─── Mock Review Comments ───────────────────────────────────────────────────

export const mockReviewComments: ReviewComment[] = [
  {
    id: 'c1c1c1c1-0001-0001-0001-000000000001',
    review_id: 'a1b2c3d4-0001-0001-0001-000000000001',
    file_path: 'src/auth/handlers.ts',
    line_number: 45,
    severity: 'critical',
    title: 'JWT secret stored in plaintext',
    explanation:
      'The JWT signing secret is hardcoded as a string literal in the source code. This exposes the secret in version control and makes token forgery trivial if the codebase is leaked.',
    why_it_matters:
      'An attacker with access to this secret can forge authentication tokens for any user, leading to complete account takeover.',
    suggestion:
      'Move the JWT secret to an environment variable and use a secrets manager in production.',
    code_snippet: "const JWT_SECRET = 'super-secret-key-12345';",
    published: true,
    created_at: '2026-07-04T14:30:30Z',
  },
  {
    id: 'c1c1c1c1-0001-0001-0001-000000000002',
    review_id: 'a1b2c3d4-0001-0001-0001-000000000001',
    file_path: 'src/auth/handlers.ts',
    line_number: 78,
    severity: 'high',
    title: 'Missing rate limiting on login endpoint',
    explanation:
      'The login endpoint accepts unlimited authentication attempts without any rate limiting or account lockout mechanism.',
    why_it_matters:
      'Without rate limiting, the endpoint is vulnerable to brute force attacks and credential stuffing.',
    suggestion:
      'Implement rate limiting using a sliding window counter. Limit to 5 attempts per minute per IP/account.',
    code_snippet: "app.post('/auth/login', loginHandler);",
    published: true,
    created_at: '2026-07-04T14:30:35Z',
  },
  {
    id: 'c1c1c1c1-0001-0001-0001-000000000003',
    review_id: 'a1b2c3d4-0001-0001-0001-000000000001',
    file_path: 'src/auth/middleware.ts',
    line_number: 12,
    severity: 'high',
    title: 'Unhandled token verification errors',
    explanation:
      'The jwt.verify() call is not wrapped in a try-catch block. If the token is malformed or expired, the server will throw an unhandled exception.',
    why_it_matters:
      'Unhandled exceptions in middleware can crash the server process or leak error details to the client.',
    suggestion: 'Wrap token verification in try-catch and return appropriate HTTP error responses.',
    code_snippet: 'const decoded = jwt.verify(token, JWT_SECRET);\nreq.user = decoded;\nnext();',
    published: true,
    created_at: '2026-07-04T14:30:40Z',
  },
  {
    id: 'c1c1c1c1-0001-0001-0001-000000000004',
    review_id: 'a1b2c3d4-0001-0001-0001-000000000001',
    file_path: 'src/auth/utils.ts',
    line_number: 15,
    severity: 'medium',
    title: 'Using MD5 for password hashing',
    explanation:
      'MD5 is a fast, broken hash function unsuitable for password storage. Modern attacks can crack MD5 hashes in seconds.',
    why_it_matters: 'If the database is compromised, all user passwords can be trivially recovered.',
    suggestion: 'Use bcrypt or argon2 for password hashing with appropriate cost factors.',
    code_snippet: "crypto.createHash('md5').update(password).digest('hex')",
    published: true,
    created_at: '2026-07-04T14:30:55Z',
  },
];

// ─── Mock Changed Files ─────────────────────────────────────────────────────

export const mockChangedFiles: ChangedFile[] = [
  { filename: 'src/auth/handlers.ts', status: 'added', additions: 156, deletions: 0, patch: '' },
  { filename: 'src/auth/middleware.ts', status: 'added', additions: 89, deletions: 0, patch: '' },
  { filename: 'src/auth/utils.ts', status: 'added', additions: 45, deletions: 0, patch: '' },
  { filename: 'src/types/auth.ts', status: 'added', additions: 34, deletions: 0, patch: '' },
  { filename: 'src/components/LoginForm.tsx', status: 'added', additions: 187, deletions: 0, patch: '' },
  { filename: 'src/components/SignupForm.tsx', status: 'added', additions: 203, deletions: 0, patch: '' },
  { filename: 'src/hooks/useAuth.ts', status: 'added', additions: 78, deletions: 0, patch: '' },
  { filename: 'src/App.tsx', status: 'modified', additions: 45, deletions: 12, patch: '' },
  { filename: 'package.json', status: 'modified', additions: 5, deletions: 1, patch: '' },
  { filename: 'README.md', status: 'modified', additions: 48, deletions: 0, patch: '' },
];

// ─── Mock Full Review ───────────────────────────────────────────────────────

export const mockReviewWithComments: ReviewWithComments = {
  review: {
    id: mockRecentReviews[0].id,
    pull_request_id: mockRecentReviews[0].pull_request_id,
    status: mockRecentReviews[0].status,
    summary: 'Found 8 issues including a critical JWT secret exposure and missing rate limiting.',
    total_comments: mockRecentReviews[0].total_findings,
    critical_count: mockRecentReviews[0].critical_count,
    high_count: mockRecentReviews[0].high_count,
    medium_count: mockRecentReviews[0].medium_count,
    low_count: mockRecentReviews[0].low_count,
    llm_model: mockRecentReviews[0].llm_model,
    tokens_used: mockRecentReviews[0].tokens_used,
    input_tokens: 3400,
    output_tokens: 800,
    cost_usd: mockRecentReviews[0].cost_usd,
    processing_time_ms: mockRecentReviews[0].processing_time_ms,
    error_message: '',
    started_at: mockRecentReviews[0].started_at,
    completed_at: mockRecentReviews[0].completed_at,
    created_at: mockRecentReviews[0].created_at,
  },
  comments: mockReviewComments,
};

// ─── Mock Activity Feed ─────────────────────────────────────────────────────

const minutesAgo = (m: number) => new Date(Date.now() - m * 60000).toISOString();

export const mockActivity: ActivityEvent[] = [
  {
    id: 'act-1', review_id: mockRecentReviews[0].id, pull_request_id: mockRecentReviews[0].pull_request_id,
    repository_name: 'acme/web-app', pr_number: 342, pr_title: 'feat: Add user authentication flow',
    step: 'publish_review', status: 'success', message: 'published 5 comments', duration_ms: 640, created_at: minutesAgo(2),
  },
  {
    id: 'act-2', review_id: mockRecentReviews[0].id, pull_request_id: mockRecentReviews[0].pull_request_id,
    repository_name: 'acme/web-app', pr_number: 342, pr_title: 'feat: Add user authentication flow',
    step: 'reflection', status: 'success', message: 'kept=6 filtered=2 tokens=980', duration_ms: 1120, created_at: minutesAgo(2),
  },
  {
    id: 'act-3', review_id: mockRecentReviews[0].id, pull_request_id: mockRecentReviews[0].pull_request_id,
    repository_name: 'acme/web-app', pr_number: 342, pr_title: 'feat: Add user authentication flow',
    step: 'agent_review', status: 'success', message: 'steps=4 tool_calls=3 findings=8 tokens=4200', duration_ms: 21850, created_at: minutesAgo(3),
  },
  {
    id: 'act-4', review_id: mockRecentReviews[0].id, pull_request_id: mockRecentReviews[0].pull_request_id,
    repository_name: 'acme/web-app', pr_number: 342, pr_title: 'feat: Add user authentication flow',
    step: 'triage', status: 'success', message: 'selected 5/12 files', duration_ms: 410, created_at: minutesAgo(3),
  },
  {
    id: 'act-5', review_id: mockRecentReviews[1].id, pull_request_id: mockRecentReviews[1].pull_request_id,
    repository_name: 'acme/api-service', pr_number: 189, pr_title: 'fix: Resolve N+1 query in orders endpoint',
    step: 'rag_index', status: 'success', message: 'indexed 4 files', duration_ms: 890, created_at: minutesAgo(6),
  },
  {
    id: 'act-6', review_id: mockRecentReviews[4].id, pull_request_id: mockRecentReviews[4].pull_request_id,
    repository_name: 'acme/api-service', pr_number: 188, pr_title: 'chore: Update dependencies',
    step: 'publish_review', status: 'failed', message: 'Rate limit exceeded', duration_ms: 120, created_at: minutesAgo(11),
  },
  {
    id: 'act-7', review_id: mockRecentReviews[3].id, pull_request_id: mockRecentReviews[3].pull_request_id,
    repository_name: 'acme/mobile-sdk', pr_number: 56, pr_title: 'feat: Add biometric login support',
    step: 'llm_review', status: 'success', message: 'tokens_used=6100 files_reviewed=8', duration_ms: 30120, created_at: minutesAgo(35),
  },
  {
    id: 'act-8', review_id: mockRecentReviews[5].id, pull_request_id: mockRecentReviews[5].pull_request_id,
    repository_name: 'acme/infra-tools', pr_number: 23, pr_title: 'feat: Terraform module for EKS cluster',
    step: 'triage', status: 'skipped', message: 'PR has 3 files — below threshold, reviewing all', duration_ms: 0, created_at: minutesAgo(62),
  },
];

// ─── Mock Execution Logs (agent trace for a review) ─────────────────────────

const secsAgo = (s: number) => new Date(Date.now() - s * 1000).toISOString();
const demoReviewId = mockRecentReviews[0].id;

export const mockExecutionLogs: ExecutionLog[] = [
  { id: 'log-1', review_id: demoReviewId, step: 'triage', status: 'success', message: 'selected 3/6 files: internal/auth/jwt.go, internal/api/handlers/login.go', duration_ms: 410, created_at: secsAgo(75) },
  { id: 'log-2', review_id: demoReviewId, step: 'rag_index', status: 'success', message: 'indexed 3 files', duration_ms: 880, created_at: secsAgo(72) },
  { id: 'log-3', review_id: demoReviewId, step: 'agent_tool', status: 'success', message: '#1 get_file_diff(internal/auth/jwt.go) → 34 additions', duration_ms: 12, created_at: secsAgo(70) },
  { id: 'log-4', review_id: demoReviewId, step: 'agent_tool', status: 'success', message: '#2 retrieve_context(where is JWT_SECRET loaded) → 2 chunks', duration_ms: 640, created_at: secsAgo(68) },
  { id: 'log-5', review_id: demoReviewId, step: 'agent_tool', status: 'success', message: '#3 get_file_contents(internal/config/config.go) → 120 lines', duration_ms: 210, created_at: secsAgo(66) },
  { id: 'log-6', review_id: demoReviewId, step: 'agent_review', status: 'success', message: 'steps=4 tool_calls=3 findings=8 tokens=4200', duration_ms: 21850, created_at: secsAgo(45) },
  { id: 'log-7', review_id: demoReviewId, step: 'reflection', status: 'success', message: 'kept=6 filtered=2 tokens=980', duration_ms: 1120, created_at: secsAgo(43) },
  { id: 'log-8', review_id: demoReviewId, step: 'publish_review', status: 'success', message: 'published 5 comments', duration_ms: 640, created_at: secsAgo(42) },
];
