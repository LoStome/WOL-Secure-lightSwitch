import assert from 'node:assert/strict';
import test from 'node:test';
import { ApiError, fetchUsers, login } from '../src/services/api.ts';

test('keeps the login endpoint and JSON payload while returning its response', async (t) => {
  const user = { id: 1, email: 'admin@example.com', is_admin: true, devices: [] };
  const fetchMock = t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ user }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  }));

  const result = await login('admin@example.com', 'password');
  const [url, init] = fetchMock.mock.calls[0]!.arguments;

  assert.equal(url, '/api/login');
  assert.equal(init?.method, 'POST');
  assert.deepEqual(JSON.parse(init?.body as string), { email: 'admin@example.com', password: 'password' });
  assert.deepEqual(result, { user });
});

test('surfaces a safe backend authorization message and status', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: 'Admin access required' }), {
    status: 403,
    headers: { 'Content-Type': 'application/json' },
  }));

  await assert.rejects(fetchUsers(), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.kind, 'http');
    assert.equal(error.status, 403);
    assert.equal(error.message, 'Admin access required');
    return true;
  });
});

test('surfaces the backend login message for unauthorized responses', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: 'Invalid email or password' }), {
    status: 401,
    headers: { 'Content-Type': 'application/json' },
  }));

  await assert.rejects(login('admin@example.com', 'wrong-password'), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.kind, 'http');
    assert.equal(error.status, 401);
    assert.equal(error.message, 'Invalid email or password');
    return true;
  });
});

test('surfaces conflict details from the backend', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: 'Cannot delete the last administrator' }), {
    status: 409,
    headers: { 'Content-Type': 'application/json' },
  }));

  await assert.rejects(fetchUsers(), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.kind, 'http');
    assert.equal(error.status, 409);
    assert.equal(error.message, 'Cannot delete the last administrator');
    return true;
  });
});

test('uses a status fallback when an error body cannot be parsed', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response('<html>error</html>', { status: 500 }));

  await assert.rejects(fetchUsers(), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.kind, 'http');
    assert.equal(error.status, 500);
    assert.equal(error.message, 'The server could not complete the request.');
    return true;
  });
});

test('includes a backend request ID in server errors', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({
    error: 'Unable to shut down device',
    request_id: 'req-123',
  }), {
    status: 500,
    headers: { 'Content-Type': 'application/json' },
  }));

  await assert.rejects(fetchUsers(), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.requestId, 'req-123');
    assert.equal(error.message, 'Unable to shut down device (Request ID: req-123)');
    return true;
  });
});

test('classifies network failures separately from HTTP errors', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => {
    throw new TypeError('Failed to fetch');
  });

  await assert.rejects(login('admin@example.com', 'password'), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.kind, 'network');
    assert.equal(error.status, undefined);
    return true;
  });
});

test('aborts a request when its timeout expires', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let requestSignal: AbortSignal | undefined;
  t.mock.method(globalThis, 'fetch', (_input, init) => {
    requestSignal = init?.signal ?? undefined;
    return new Promise<Response>((_resolve, reject) => {
      requestSignal?.addEventListener('abort', () => {
        reject(new DOMException('The operation was aborted.', 'AbortError'));
      }, { once: true });
    });
  });

  const request = login('admin@example.com', 'password');
  t.mock.timers.tick(60_000);

  await assert.rejects(request, (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.kind, 'timeout');
    return true;
  });
  assert.equal(requestSignal?.aborted, true);
});
