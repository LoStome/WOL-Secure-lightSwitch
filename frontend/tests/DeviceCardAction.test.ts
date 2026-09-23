import assert from 'node:assert/strict';
import test from 'node:test';
import { DEVICE_ACTION_TIMEOUT_MS, DeviceActionTracker } from '../src/components/deviceActionState.ts';

test('a matching ping clears the pending action and its timeout', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const tracker = new DeviceActionTracker(() => {});

  tracker.start(true);
  tracker.commandSent();
  tracker.confirm(true);
  t.mock.timers.tick(DEVICE_ACTION_TIMEOUT_MS);

  assert.deepEqual(tracker.state, { status: 'idle' });
  tracker.dispose();
});

test('a missing ping confirmation times out and allows another attempt', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const tracker = new DeviceActionTracker(() => {});

  tracker.start(true);
  tracker.commandSent();
  t.mock.timers.tick(DEVICE_ACTION_TIMEOUT_MS);

  assert.deepEqual(tracker.state, { status: 'timedOut', expectedState: true });

  tracker.start(false);
  assert.deepEqual(tracker.state, { status: 'sending', expectedState: false });
  tracker.dispose();
});

test('a failed action clears pending work and exposes an error state', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const tracker = new DeviceActionTracker(() => {});

  tracker.start(true);
  tracker.fail();
  t.mock.timers.tick(DEVICE_ACTION_TIMEOUT_MS);

  assert.deepEqual(tracker.state, { status: 'error' });
  tracker.dispose();
});
