package net

import (
	"bytes"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const (
	OutOfBandHeader  = "\xff\xff\xff\xff"
	GetInfoCommand   = "getinfo"
	GetStatusCommand = "getstatus"
)

// commandBufferSize defines an appropriate buffer size for command responses
const commandBufferSize = 8192

// Use a sync.Pool to reuse buffers and avoid GC pressure
var commandBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, commandBufferSize)
	},
}

// connectionTimeout defines how long to wait for a response
const connectionTimeout = 3 * time.Second

func SendCommand(addr, cmd string) ([]byte, error) {
	// Resolve the address once
	raddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve UDP address")
	}
	
	// Use a dialer instead of ListenPacket for better performance
	conn, err := net.DialUDP("udp4", nil, raddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create UDP connection")
	}
	defer conn.Close()
	
	// Set deadline immediately to avoid hanging
	if err := conn.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, errors.Wrap(err, "failed to set deadline")
	}
	
	// Create the command once with proper capacity estimation
	command := OutOfBandHeader + cmd
	
	// Send the command
	if _, err := conn.Write([]byte(command)); err != nil {
		return nil, errors.Wrap(err, "failed to send command")
	}
	
	// Get a buffer from the pool
	bufferInterface := commandBufferPool.Get()
	buffer := bufferInterface.([]byte)
	defer commandBufferPool.Put(bufferInterface)
	
	// Read the response
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}
	
	// Return a copy of the relevant part of the buffer
	// (we need to copy here so we can return the buffer to the pool)
	result := make([]byte, n)
	copy(result, buffer[:n])
	return result, nil
}

// parseMap optimized for performance
func parseMap(data []byte) map[string]string {
	// Pre-allocate capacity for better performance on single line processing
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		data = data[i+1:]
	}
	
	// Remove any prefixes and suffixes in one operation
	data = bytes.TrimPrefix(data, []byte("\\"))
	data = bytes.TrimSuffix(data, []byte("\n"))
	
	// Use a more efficient split
	parts := bytes.Split(data, []byte("\\"))
	partCount := len(parts)
	
	// Pre-allocate map with correct capacity
	// This avoids rehashing and map growth
	expectedSize := (partCount - 1) / 2
	m := make(map[string]string, expectedSize)
	
	// Process in pairs to avoid bounds checking in loop
	for i := 0; i < partCount-1; i += 2 {
		key := string(parts[i])
		val := string(parts[i+1])
		m[key] = val
	}
	
	return m
}

type Player struct {
	Name  string
	Ping  int
	Score int
}

// parsePlayers optimized for performance
func parsePlayers(data []byte) ([]Player, error) {
	// Count number of lines to pre-allocate players slice
	lineCount := bytes.Count(data, []byte("\n")) + 1
	players := make([]Player, 0, lineCount)
	
	// Split players data by newline
	playerLines := bytes.Split(data, []byte("\n"))
	
	for _, player := range playerLines {
		// Skip empty lines
		if len(player) == 0 {
			continue
		}
		
		// Efficiently split the player data
		parts := bytes.SplitN(player, []byte(" "), 3)
		if len(parts) != 3 {
			continue
		}
		
		// Process score (string conversion only when needed)
		scoreBytes := parts[0]
		score, err := strconv.Atoi(string(scoreBytes))
		if err != nil {
			continue // Skip this player instead of failing completely
		}
		
		// Process ping (string conversion only when needed)
		pingBytes := parts[1]
		ping, err := strconv.Atoi(string(pingBytes))
		if err != nil {
			continue // Skip this player instead of failing completely
		}
		
		// Process name (string conversion only when needed)
		nameBytes := parts[2]
		name, err := strconv.Unquote(string(nameBytes))
		if err != nil {
			continue // Skip this player instead of failing completely
		}
		
		// Add player to slice (pre-allocated so this is efficient)
		players = append(players, Player{
			Name:  name,
			Ping:  ping,
			Score: score,
		})
	}
	
	return players, nil
}

func GetInfo(addr string) (map[string]string, error) {
	resp, err := SendCommand(addr, GetInfoCommand)
	if err != nil {
		return nil, err
	}
	return parseMap(resp), nil
}

type StatusResponse struct {
	Configuration map[string]string
	Players       []Player
}

// Pre-allocate common response objects
var emptyPlayerSlice = make([]Player, 0)

func GetStatus(addr string) (*StatusResponse, error) {
	// Fetch response with better error handling
	resp, err := SendCommand(addr, GetStatusCommand)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send status command")
	}
	
	// Trim once for efficiency
	data := bytes.TrimSuffix(resp, []byte("\n"))
	
	// Split efficiently with upper bound to avoid excessive allocations
	parts := bytes.SplitN(data, []byte("\n"), 3)
	
	// Pre-allocate response for better performance
	status := &StatusResponse{}
	
	switch len(parts) {
	case 2:
		// Parse configuration data
		status.Configuration = parseMap(parts[1])
		// Use pre-allocated empty slice for better performance
		status.Players = emptyPlayerSlice
		return status, nil
	case 3:
		// Parse configuration data
		status.Configuration = parseMap(parts[1])
		// Parse player data with resilient error handling
		players, err := parsePlayers(parts[2])
		if err != nil {
			// Don't fail completely if player parsing fails
			status.Players = emptyPlayerSlice
		} else {
			status.Players = players
		}
		return status, nil
	default:
		return nil, errors.Errorf("invalid response format: %q", resp)
	}
}
