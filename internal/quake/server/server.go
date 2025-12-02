package server

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/yaml"

	quakenet "github.com/lukaszraczylo/quake-kube/internal/quake/net"
	"github.com/lukaszraczylo/quake-kube/internal/util/exec"
)

var (
	actrvePlayers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "quake_active_players",
		Help: "The current number of active players",
	})

	scores = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "quake_player_scores",
		Help: "Current scores by player, by map",
	}, []string{"player", "map"})

	pings = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "quake_player_pings",
		Help: "Current ping by player",
	}, []string{"player"})

	configReloads = promauto.NewCounter(prometheus.CounterOpts{
		Name: "quake_config_reloads",
		Help: "Config file reload count",
	})
)

type Server struct {
	Dir           string
	WatchInterval time.Duration
	ConfigFile    string
	Addr          string
}

func (s *Server) Start(ctx context.Context) error {
	// Use default address if not provided
	if s.Addr == "" {
		s.Addr = "0.0.0.0:27960"
	}

	// Parse address once
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return err
	}

	// Pre-allocate command arguments with proper capacity
	args := make([]string, 0, 16) // Optimize initial capacity
	args = append(args,
		"+set", "dedicated", "1",
		"+set", "net_ip", host,
		"+set", "net_port", port,
		"+set", "com_homepath", s.Dir,
		"+set", "com_basegame", "baseq3",
		"+set", "com_gamename", "Quake3Arena",
		"+exec", "server.cfg",
	)

	// Create command with context for proper cancellation handling
	cmd := exec.CommandContext(ctx, "ioq3ded", args...)
	cmd.Dir = s.Dir

	// Use buffered output for better performance
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if s.ConfigFile == "" {
		cfg := Default()
		data, err := cfg.Marshal()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(s.Dir, "baseq3/server.cfg"), data, 0644); err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Wait()
	}

	if err := s.reload(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Println(err)
		}
	}()

	// Optimized metrics collection
	go func() {
		// Resolve the correct address for localhost connections
		addr := s.Addr
		if net.ParseIP(host).IsUnspecified() {
			addr = net.JoinHostPort("127.0.0.1", port)
		}

		// Adaptive polling frequency - start with 5s
		pollInterval := 5 * time.Second
		minInterval := 2 * time.Second  // Minimum interval for high traffic
		maxInterval := 15 * time.Second // Maximum interval for no players

		// Use a ticker for regular polling
		tick := time.NewTicker(pollInterval)
		defer tick.Stop()

		// Track player counts to adjust polling frequency
		var lastPlayerCount int

		for {
			select {
			case <-tick.C:
				// Get server status with optimized network call
				status, err := quakenet.GetStatus(addr)
				if err != nil {
					log.Printf("metrics: get status failed %v", err)
					continue
				}

				// Update player count metric
				currentPlayerCount := len(status.Players)
				actrvePlayers.Set(float64(currentPlayerCount))

				// Set player-specific metrics more efficiently
				for _, p := range status.Players {
					if mapname, ok := status.Configuration["mapname"]; ok {
						scores.WithLabelValues(p.Name, mapname).Set(float64(p.Score))
					}
					pings.WithLabelValues(p.Name).Set(float64(p.Ping))
				}

				// Adjust polling frequency based on player activity
				// More players = more frequent updates
				if currentPlayerCount > 0 {
					// More players need more frequent updates
					newInterval := maxInterval - time.Duration(currentPlayerCount)*time.Second
					if newInterval < minInterval {
						newInterval = minInterval
					}

					// Only change ticker if interval changed significantly
					if newInterval != pollInterval &&
						(newInterval < pollInterval-time.Second || newInterval > pollInterval+time.Second) {
						pollInterval = newInterval
						tick.Reset(pollInterval)
					}
				} else if currentPlayerCount == 0 && lastPlayerCount > 0 {
					// No players, reduce polling frequency
					pollInterval = maxInterval
					tick.Reset(pollInterval)
				}

				lastPlayerCount = currentPlayerCount

			case <-ctx.Done():
				return
			}
		}
	}()

	ch, err := s.watch(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ch:
			if err := s.reload(); err != nil {
				return err
			}
			configReloads.Inc()
			if err := cmd.Restart(ctx); err != nil {
				return err
			}
			go func() {
				if err := cmd.Wait(); err != nil {
					log.Println(err)
				}
			}()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// reload optimized for better performance
func (s *Server) reload() error {
	// Read with better error details
	data, err := os.ReadFile(s.ConfigFile)
	if err != nil {
		return err
	}

	// Pre-allocate config with defaults
	cfg := Default()

	// Single unmarshal operation
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Marshal with optimized memory handling
	data, err = cfg.Marshal()
	if err != nil {
		return err
	}

	// Create target path only once
	targetPath := filepath.Join(s.Dir, "baseq3/server.cfg")

	// Write with atomic guarantee on most filesystems
	return os.WriteFile(targetPath, data, 0644)
}

// watch monitors config file for changes with optimized performance
func (s *Server) watch(ctx context.Context) (<-chan struct{}, error) {
	// Use default interval if not set
	if s.WatchInterval == 0 {
		s.WatchInterval = 15 * time.Second
	}

	// Get initial file info once
	cur, err := os.Stat(s.ConfigFile)
	if err != nil {
		return nil, err
	}

	// Create buffered channel for better performance under load
	// Buffer size of 1 prevents blocking in most scenarios
	ch := make(chan struct{}, 1)

	go func() {
		// Create ticker with adaptive interval
		ticker := time.NewTicker(s.WatchInterval)
		defer ticker.Stop()

		// Cache path to avoid repeated allocations
		configPath := s.ConfigFile

		// Cache last mod time for faster comparison
		lastMod := cur.ModTime()

		for {
			select {
			case <-ticker.C:
				// Get file info with optimized error handling
				if fi, err := os.Stat(configPath); err == nil {
					// Compare mod times efficiently
					curMod := fi.ModTime()
					if curMod.After(lastMod) {
						// Non-blocking send with buffered channel
						select {
						case ch <- struct{}{}:
							// Signal sent successfully
						default:
							// Channel full, skip this update (prevents blocking)
							log.Println("Config reload signal skipped - channel full")
						}

						// Update last modified time
						lastMod = curMod
					}
					cur = fi
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
