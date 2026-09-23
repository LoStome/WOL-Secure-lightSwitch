package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPingManagerLimitsAndCancelsProbes(t *testing.T) {
	hosts := []Host{
		{ID: "host1", IP: "192.0.2.1", PingInterval: 1},
		{ID: "host2", IP: "192.0.2.2", PingInterval: 1},
		{ID: "host3", IP: "192.0.2.3", PingInterval: 1},
		{ID: "host4", IP: "192.0.2.4", PingInterval: 1},
		{ID: "host5", IP: "192.0.2.5", PingInterval: 1},
	}
	started := make(chan string, len(hosts)+1)
	release := make(chan struct{})
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var running atomic.Int32
	var maximum atomic.Int32
	probe := func(ctx context.Context, ip string) bool {
		current := running.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- ip
		select {
		case <-release:
		case <-ctx.Done():
		}
		running.Add(-1)
		return false
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPingManager(ctx, func() ([]Host, error) { return hosts, nil }, probe, ticks)
	}()
	for range 4 {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("four initial probes did not start")
		}
	}
	select {
	case ip := <-started:
		t.Fatalf("extra probe started while four were active: %s", ip)
	case <-time.After(100 * time.Millisecond):
	}
	if maximum.Load() != 4 {
		t.Fatalf("maximum concurrent probes = %d, want 4", maximum.Load())
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ping manager did not cancel and wait for active probes")
	}
	if running.Load() != 0 {
		t.Fatalf("%d probes remain after shutdown", running.Load())
	}
}

func TestPingManagerDoesNotOverlapOneHost(t *testing.T) {
	started := make(chan struct{}, 2)
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPingManager(ctx,
			func() ([]Host, error) {
				return []Host{{ID: "rel04-overlap", IP: "192.0.2.8", PingInterval: 1}}, nil
			},
			func(ctx context.Context, _ string) bool {
				started <- struct{}{}
				<-ctx.Done()
				return false
			}, ticks)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("initial probe did not start")
	}
	time.Sleep(1100 * time.Millisecond)
	select {
	case ticks <- time.Now():
	case <-time.After(2 * time.Second):
		t.Fatal("ping manager did not check the next interval")
	}
	select {
	case <-started:
		t.Fatal("a second probe started for a host with one still active")
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ping manager did not stop")
	}
}

func TestPingManagerRemovesHostStateAndAllowsReaddedHost(t *testing.T) {
	const id = "rel04-removed"
	hostStates.Lock()
	hostStates.Status[id] = HostState{Online: true}
	hostStates.Unlock()
	defer func() {
		hostStates.Lock()
		delete(hostStates.Status, id)
		hostStates.Unlock()
	}()

	var configured atomic.Bool
	configured.Store(true)
	var calls atomic.Int32
	started := make(chan int32, 2)
	firstDone := make(chan struct{})
	ticks := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPingManager(ctx,
			func() ([]Host, error) {
				if !configured.Load() {
					return nil, nil
				}
				return []Host{{ID: id, IP: "192.0.2.9", PingInterval: 3600}}, nil
			},
			func(ctx context.Context, _ string) bool {
				call := calls.Add(1)
				started <- call
				<-ctx.Done()
				if call == 1 {
					close(firstDone)
					return true
				}
				return false
			}, ticks)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("initial probe did not start")
	}
	configured.Store(false)
	ticks <- time.Now()
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("probe was not canceled when the host was removed")
	}
	configured.Store(true)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ticks <- time.Now():
		default:
		}
		select {
		case call := <-started:
			if call != 2 {
				t.Fatalf("unexpected probe number %d", call)
			}
			hostStates.RLock()
			_, exists := hostStates.Status[id]
			hostStates.RUnlock()
			if exists {
				t.Fatal("removed host state was recreated by its canceled probe")
			}
			cancel()
			<-done
			return
		case <-deadline:
			t.Fatal("re-added host did not get an immediate probe")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
