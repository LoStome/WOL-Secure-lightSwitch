import assert from 'node:assert/strict';
import test from 'node:test';
import { HOST_POLL_INTERVAL_MS, startHostPolling, type HostPollingState } from '../src/components/hostPolling.ts';
import { fetchHosts } from '../src/services/api.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

async function flushPromiseJobs() {
  await Promise.resolve();
  await Promise.resolve();
}

test('does not overlap host requests and schedules the next poll after completion', (t) => {
  t.mock.timers.enable({ apis: ['setInterval', 'setTimeout'] });
  const firstRequest = deferred<string[]>();
  let requestCount = 0;
  const stop = startHostPolling(() => {
    requestCount += 1;
    return firstRequest.promise;
  }, () => {});
  t.after(stop);

  assert.equal(requestCount, 1);
  t.mock.timers.tick(HOST_POLL_INTERVAL_MS);
  assert.equal(requestCount, 1);

  firstRequest.resolve(['host-1']);
  return flushPromiseJobs().then(() => {
    t.mock.timers.tick(HOST_POLL_INTERVAL_MS - 1);
    assert.equal(requestCount, 1);
    t.mock.timers.tick(1);
    assert.equal(requestCount, 2);
  });
});

test('aborts the active request and stops polling during cleanup', async (t) => {
  t.mock.timers.enable({ apis: ['setInterval', 'setTimeout'] });
  const pendingRequest = deferred<string[]>();
  const states: HostPollingState<string>[] = [];
  let signal: AbortSignal | undefined;
  let requestCount = 0;
  const stop = startHostPolling((requestSignal) => {
    signal = requestSignal;
    requestCount += 1;
    return pendingRequest.promise;
  }, (state) => states.push(state));

  stop();
  assert.equal(signal?.aborted, true);
  t.mock.timers.tick(HOST_POLL_INTERVAL_MS * 2);
  assert.equal(requestCount, 1);
  pendingRequest.resolve(['late-host']);
  await flushPromiseJobs();
  assert.deepEqual(states, []);
});

test('clears a previous error after a later successful poll', async (t) => {
  t.mock.timers.enable({ apis: ['setInterval', 'setTimeout'] });
  const states: HostPollingState<string>[] = [];
  let requestCount = 0;
  const stop = startHostPolling(async () => {
    requestCount += 1;
    if (requestCount === 1) {
      throw new Error('temporarily unavailable');
    }
    return ['host-1'];
  }, (state) => states.push(state));
  t.after(stop);

  await flushPromiseJobs();
  assert.equal(states.at(-1)?.error, 'temporarily unavailable');
  t.mock.timers.tick(HOST_POLL_INTERVAL_MS);
  await flushPromiseJobs();

  assert.deepEqual(states.at(-1), {
    hosts: ['host-1'],
    loading: false,
    error: null,
  });
});

test('aborts the hosts request when its caller signal is aborted', async (t) => {
  const controller = new AbortController();
  let requestSignal: AbortSignal | undefined;
  const fetchMock = t.mock.method(globalThis, 'fetch', (_input, init) => {
    requestSignal = init?.signal ?? undefined;
    return new Promise<Response>((_resolve, reject) => {
      requestSignal?.addEventListener('abort', () => {
        reject(new DOMException('The operation was aborted.', 'AbortError'));
      }, { once: true });
    });
  });

  const request = fetchHosts(controller.signal);
  controller.abort();

  await assert.rejects(request, { name: 'AbortError' });
  assert.equal(fetchMock.mock.calls[0]?.arguments[1]?.signal, requestSignal);
  assert.equal(requestSignal?.aborted, true);
});
