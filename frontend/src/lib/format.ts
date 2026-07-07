// Shared display formatters.

/** Compact "Xm/Xh/Xd ago" relative time. */
export function formatTimeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  return `${days}d ago`;
}

/** Turn a snake_case pipeline step into a Title-cased label. */
export function prettyStep(step: string): string {
  return step.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}
