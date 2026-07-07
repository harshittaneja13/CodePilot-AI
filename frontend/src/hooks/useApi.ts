import { useState, useEffect, useCallback, useRef } from 'react';

interface UseApiResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
  silentRefetch: () => void; // refetches in the background without triggering the loading state
}

export function useApi<T>(
  fetchFn: () => Promise<T>,
  fallback?: T,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  deps: any[] = [],
): UseApiResult<T> {
  const [data, setData] = useState<T | null>(fallback ?? null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const fetchRef = useRef(fetchFn);
  fetchRef.current = fetchFn;

  const fetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await fetchRef.current();
      setData(result);
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : 'An unknown error occurred';
      setError(message);
      if (fallback) {
        setData(fallback);
      }
    } finally {
      setLoading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fallback, ...deps]);

  // Fetches fresh data without changing the loading state.
  // Use this for background polling so the UI doesn't flash a skeleton on every tick.
  const silentFetch = useCallback(async () => {
    try {
      const result = await fetchRef.current();
      setData(result);
    } catch {
      // silently ignore errors during background polling
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fallback, ...deps]);

  useEffect(() => {
    fetch();
  }, [fetch]);

  return { data, loading, error, refetch: fetch, silentRefetch: silentFetch };
}
