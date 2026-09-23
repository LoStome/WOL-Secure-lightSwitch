export const DEVICE_ACTION_TIMEOUT_MS = 30_000;

export type DeviceActionState =
  | { status: 'idle' }
  | { status: 'sending'; expectedState: boolean }
  | { status: 'waiting'; expectedState: boolean }
  | { status: 'timedOut'; expectedState: boolean }
  | { status: 'error' };

export class DeviceActionTracker {
  state: DeviceActionState = { status: 'idle' };
  private timeoutId: ReturnType<typeof setTimeout> | undefined;
  private readonly onChange: (state: DeviceActionState) => void;

  constructor(onChange: (state: DeviceActionState) => void) {
    this.onChange = onChange;
  }

  start(expectedState: boolean): void {
    this.clearTimeout();
    this.update({ status: 'sending', expectedState });
  }

  commandSent(): void {
    if (this.state.status !== 'sending') return;
    const expectedState = this.state.expectedState;
    this.update({ status: 'waiting', expectedState });
    this.timeoutId = setTimeout(() => {
      this.timeoutId = undefined;
      if (this.state.status === 'waiting') {
        this.update({ status: 'timedOut', expectedState });
      }
    }, DEVICE_ACTION_TIMEOUT_MS);
  }

  confirm(actualState: boolean): void {
    if (
      (this.state.status !== 'sending' && this.state.status !== 'waiting' && this.state.status !== 'timedOut') ||
      this.state.expectedState !== actualState
    ) {
      return;
    }

    this.clearTimeout();
    this.update({ status: 'idle' });
  }

  fail(): void {
    this.clearTimeout();
    this.update({ status: 'error' });
  }

  dispose(): void {
    this.clearTimeout();
  }

  private update(state: DeviceActionState): void {
    this.state = state;
    this.onChange(state);
  }

  private clearTimeout(): void {
    if (this.timeoutId !== undefined) {
      clearTimeout(this.timeoutId);
      this.timeoutId = undefined;
    }
  }
}
