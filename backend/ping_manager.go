package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	maxConcurrentPings = 4
	pingCheckInterval  = 5 * time.Second
	pingTimeout        = 2 * time.Second
)

type pingFlight struct {
	ip     string
	cancel context.CancelFunc
	valid  bool
}

type pingResult struct {
	id     string
	online bool
}

func StartPingManager(ctx context.Context) {
	fmt.Println("Ping Manager Started...")
	ticker := time.NewTicker(pingCheckInterval)
	defer ticker.Stop()
	runPingManager(ctx, LoadHosts, isOnlineContext, ticker.C)
}

func runPingManager(
	ctx context.Context,
	loadHosts func() ([]Host, error),
	probe func(context.Context, string) bool,
	ticks <-chan time.Time,
) {
	lastPingTimes := make(map[string]time.Time)
	inFlight := make(map[string]*pingFlight)
	results := make(chan pingResult, maxConcurrentPings)
	var workers sync.WaitGroup

	checkHosts := func() {
		hosts, err := loadHosts()
		if err != nil {
			log.Print("PingManager: error loading hosts")
			return
		}
		configured := make(map[string]Host, len(hosts))
		for _, host := range hosts {
			configured[host.ID] = host
		}
		for id := range lastPingTimes {
			if _, exists := configured[id]; !exists {
				delete(lastPingTimes, id)
			}
		}
		for id, flight := range inFlight {
			host, exists := configured[id]
			if !exists || host.IP != flight.ip {
				flight.valid = false
				flight.cancel()
				delete(lastPingTimes, id)
			}
		}
		hostStates.Lock()
		for id := range hostStates.Status {
			if _, exists := configured[id]; !exists {
				delete(hostStates.Status, id)
			}
		}
		hostStates.Unlock()

		now := time.Now()
		for _, host := range hosts {
			if ctx.Err() != nil || len(inFlight) == maxConcurrentPings {
				break
			}
			if _, active := inFlight[host.ID]; active {
				continue
			}
			interval := host.PingInterval
			if interval <= 0 {
				interval = 60
			}
			if last, exists := lastPingTimes[host.ID]; exists && now.Sub(last) < time.Duration(interval)*time.Second {
				continue
			}
			lastPingTimes[host.ID] = now
			probeCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			inFlight[host.ID] = &pingFlight{ip: host.IP, cancel: cancel, valid: true}
			workers.Add(1)
			go func(host Host) {
				defer workers.Done()
				defer cancel()
				results <- pingResult{id: host.ID, online: probe(probeCtx, host.IP)}
			}(host)
		}
	}

	checkHosts()
	for {
		select {
		case <-ctx.Done():
			for _, flight := range inFlight {
				flight.cancel()
			}
			workers.Wait()
			return
		case <-ticks:
			checkHosts()
		case result := <-results:
			flight, exists := inFlight[result.id]
			if !exists {
				continue
			}
			delete(inFlight, result.id)
			flight.cancel()
			if !flight.valid {
				continue
			}
			hostStates.Lock()
			state := hostStates.Status[result.id]
			state.Online = result.online
			if result.online {
				state.LastPinged = time.Now().Format("15:04:05")
			}
			hostStates.Status[result.id] = state
			hostStates.Unlock()
		}
	}
}
