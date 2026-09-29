export interface HostPollingState<T> {
  hosts: T[];
  loading: boolean;
  error: string | null;
}

export const HOST_POLL_INTERVAL_MS = 10_000;

export function startHostPolling<T>(
  fetchHosts: (signal: AbortSignal) => Promise<T[]>,
  onState: (state: HostPollingState<T>) => void,
  intervalMs = HOST_POLL_INTERVAL_MS,
): () => void {
  let state: HostPollingState<T> = { hosts: [], loading: true, error: null };
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let activeController: AbortController | undefined;

  const publish = (update: Partial<HostPollingState<T>>) => {
    if (stopped) return;
    state = { ...state, ...update };
    onState(state);
  };

  const loadHosts = async (): Promise<void> => {
    if (stopped) return;
    const controller = new AbortController();
    activeController = controller;

    try {
      const hosts = await fetchHosts(controller.signal);
      if (!stopped && !controller.signal.aborted) {
        publish({ hosts: hosts || [], error: null });
      }
    } catch (error) {
      if (!stopped && !controller.signal.aborted) {
        publish({ error: error instanceof Error ? error.message : 'Error fetching hosts' });
      }
    } finally {
      if (activeController === controller) activeController = undefined;
      if (!stopped && !controller.signal.aborted) {
        publish({ loading: false });
        timer = setTimeout(() => {
          timer = undefined;
          void loadHosts();
        }, intervalMs);
      }
    }
  };

  void loadHosts();
  return () => {
    if (stopped) return;
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
    timer = undefined;
    activeController?.abort();
    activeController = undefined;
  };
}
