package http

import (
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Shared optimized HTTP client for reuse
var (
	// Use shorter timeouts for better response times
	defaultClient = &http.Client{
		Timeout: 2 * time.Minute,
		Transport: &http.Transport{
			// Increase max idle connections for connection reuse
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			// Use shorter timeouts
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			// Shorter timeouts for TLS handshake
			TLSHandshakeTimeout: 5 * time.Second,
			// Enable HTTP/2 for better performance
			ForceAttemptHTTP2: true,
			// Disable compression handling for faster processing
			DisableCompression: true,
		},
	}

	// Short timeout client for polling operations
	pollingClient = &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 10,
			DialContext: (&net.Dialer{
				Timeout:   1 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 1 * time.Second,
			ForceAttemptHTTP2:   true,
			DisableCompression:  true,
		},
	}

	// Buffer pool for read operations
	bufferPool = sync.Pool{
		New: func() interface{} {
			// 32KB is a good default size for most HTTP responses
			return make([]byte, 32*1024)
		},
	}
)

// GetBody fetches the body of a URL with optimized performance
func GetBody(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}
	
	// Add performance-oriented headers
	req.Header.Set("Accept-Encoding", "identity") // Skip decompression overhead
	
	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("cannot get url %q: %v", url, http.StatusText(resp.StatusCode))
	}

	// Use ReadAll with a buffer from the pool for better memory efficiency
	bufInterface := bufferPool.Get()
	buffer := bufInterface.([]byte)
	defer bufferPool.Put(bufInterface)

	// Grow a separate result buffer based on content length if available
	var result []byte
	if resp.ContentLength > 0 {
		result = make([]byte, 0, resp.ContentLength)
	} else {
		result = make([]byte, 0, len(buffer))
	}

	// Read chunks efficiently
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			result = append(result, buffer[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "error reading response body")
		}
	}
	
	return result, nil
}

// GetUntil polls a URL until it's available or the stop channel is closed
func GetUntil(url string, stop <-chan struct{}) error {
	// Use exponential backoff for more efficient polling
	backoff := 50 * time.Millisecond
	maxBackoff := 1 * time.Second
	
	for {
		select {
		case <-stop:
			return errors.Errorf("not available: %q", url)
		default:
			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				time.Sleep(backoff)
				// Exponential backoff with a cap
				if backoff < maxBackoff {
					backoff *= 2
				}
				continue
			}
			
			// Use HEAD method instead of GET for faster polling
			req.Method = http.MethodHead
			
			resp, err := pollingClient.Do(req)
			if err != nil {
				time.Sleep(backoff)
				// Exponential backoff with a cap
				if backoff < maxBackoff {
					backoff *= 2
				}
				continue
			}
			resp.Body.Close()
			return nil
		}
	}
}
