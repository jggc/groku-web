package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type grokuConfig struct {
	Address   string `json:"address"`
	Timestamp int64  `json:"timestamp"`
}

type Device struct {
	Location string `json:"location"`
	DeviceInfo
	ActiveApp string `json:"activeApp,omitempty"`
}

type deviceStore struct {
	mu       sync.Mutex
	location string
	path     string
}

func newDeviceStore(path string) *deviceStore {
	return &deviceStore{path: path}
}

func (s *deviceStore) get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.location
}

func (s *deviceStore) set(loc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.location = normalizeLocation(loc)
	s.saveLocked()
}

func (s *deviceStore) loadOrDiscover() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.location != "" {
		return s.location, nil
	}

	if cfg, err := readConfig(s.path); err == nil && cfg.Address != "" {
		// cache valid for 5 minutes for serve; CLI still refreshes more often via ensure
		if time.Now().Unix()-cfg.Timestamp < 300 {
			s.location = normalizeLocation(cfg.Address)
			return s.location, nil
		}
	}

	locs, err := discoverRokus(3 * time.Second)
	if err != nil {
		return "", err
	}
	if len(locs) == 0 {
		return "", fmt.Errorf("could not find a Roku on the network")
	}
	s.location = normalizeLocation(locs[0])
	s.saveLocked()
	return s.location, nil
}

func (s *deviceStore) ensure() (string, error) {
	s.mu.Lock()
	loc := s.location
	s.mu.Unlock()
	if loc != "" {
		return loc, nil
	}
	return s.loadOrDiscover()
}

func (s *deviceStore) saveLocked() {
	if s.path == "" || s.location == "" {
		return
	}
	b, err := json.Marshal(grokuConfig{
		Address:   s.location,
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, b, 0o644)
}

func readConfig(path string) (grokuConfig, error) {
	var cfg grokuConfig
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&cfg)
	return cfg, err
}

func enrichDevice(loc string) (*Device, error) {
	info, err := queryDeviceInfo(loc)
	if err != nil {
		return nil, err
	}
	return &Device{
		Location:   normalizeLocation(loc),
		DeviceInfo: *info,
		ActiveApp:  queryActiveApp(loc),
	}, nil
}
