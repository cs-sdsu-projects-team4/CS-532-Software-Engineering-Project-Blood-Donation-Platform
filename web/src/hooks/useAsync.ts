import { useCallback, useEffect, useState } from "react";

export interface AsyncState<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
  /** Re-run the async function (e.g. a Retry button). */
  reload: () => void;
}

/**
 * Runs an async function on mount (and on reload), tracking loading/error/data.
 * Pass a memoised `fn` (useCallback) so it only re-runs when its inputs change;
 * results from a stale run are ignored if the inputs change mid-flight.
 */
export function useAsync<T>(fn: () => Promise<T>): AsyncState<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    fn()
      .then((result) => {
        if (active) {
          setData(result);
        }
      })
      .catch((err: unknown) => {
        if (active) {
          setError(err instanceof Error ? err.message : "Something went wrong");
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [fn, nonce]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  return { data, error, loading, reload };
}
