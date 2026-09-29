package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Loader struct {
	sync.Mutex
	workingDir  string
	path        string
	modTime     time.Time
	size        int64
	checked     bool
	hosts       []Host
	loaded      bool
	reloadError error
}

// LoadHosts returns the latest complete configuration, including when a reload fails.
func (l *Loader) LoadHosts() ([]Host, error) {
	hosts, err := l.LoadHostsWithStatus()
	if hosts != nil {
		return hosts, nil
	}
	return nil, err
}

// LoadHostsWithStatus also reports errors in the current file for /healthz.
func (l *Loader) LoadHostsWithStatus() ([]Host, error) {
	l.Lock()
	defer l.Unlock()

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if workingDir != l.workingDir {
		l.workingDir = workingDir
		l.path = ""
		l.checked = false
		l.hosts = nil
		l.loaded = false
		l.reloadError = nil
	}

	path := filepath.Join(workingDir, "data", "hosts.yaml")
	info, err := os.Stat(path)
	if err != nil {
		path = filepath.Join(workingDir, "..", "data", "hosts.yaml")
		info, err = os.Stat(path)
	}
	if err != nil {
		l.reloadError = fmt.Errorf("stat hosts.yaml: %w", err)
		l.checked = false
		return copyHosts(l.hosts, l.loaded), l.reloadError
	}
	if !info.Mode().IsRegular() {
		l.reloadError = errors.New("hosts.yaml is not a regular file")
		l.checked = false
		return copyHosts(l.hosts, l.loaded), l.reloadError
	}
	if l.checked && l.path == path &&
		l.modTime.Equal(info.ModTime()) && l.size == info.Size() {
		return copyHosts(l.hosts, l.loaded), l.reloadError
	}

	l.path = path
	l.modTime = info.ModTime()
	l.size = info.Size()
	l.checked = true
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed []Host
		parsed, err = ParseHosts(data)
		if err == nil {
			l.hosts = parsed
			l.loaded = true
		}
	} else {
		l.checked = false
	}
	l.reloadError = err
	return copyHosts(l.hosts, l.loaded), err
}

func copyHosts(hosts []Host, loaded bool) []Host {
	if !loaded {
		return nil
	}
	copyOfHosts := make([]Host, len(hosts))
	copy(copyOfHosts, hosts)
	for i := range copyOfHosts {
		copyOfHosts[i].SkipInterfaces = append([]string(nil), hosts[i].SkipInterfaces...)
	}
	return copyOfHosts
}
