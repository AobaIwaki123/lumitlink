package litlink

import (
	"context"
	"sync"
	"time"
)

// CacheState is the cache result attached to an API request.
type CacheState string

const (
	CacheMiss CacheState = "MISS"
	CacheHit  CacheState = "HIT"
)

type cacheEntry struct {
	response Response
	storedAt time.Time
}

// ProfileService adds a bounded-time in-memory cache to the Lit.link client.
type ProfileService struct {
	client *Client
	ttl    time.Duration
	now    func() time.Time

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// NewProfileService creates a profile service. A zero TTL disables cache hits.
func NewProfileService(client *Client, ttl time.Duration) *ProfileService {
	return &ProfileService{
		client: client,
		ttl:    ttl,
		now:    time.Now,
		cache:  make(map[string]cacheEntry),
	}
}

// Get returns one profile and whether it came from the in-memory cache.
func (s *ProfileService) Get(ctx context.Context, username string, refresh bool) (Response, CacheState, error) {
	validated, err := ValidateUsername(username)
	if err != nil {
		return Response{}, CacheMiss, err
	}
	now := s.now()
	if !refresh && s.ttl > 0 {
		s.mu.RLock()
		entry, ok := s.cache[validated]
		s.mu.RUnlock()
		if ok && now.Sub(entry.storedAt) < s.ttl {
			response := entry.response
			response.Cached = true
			response.Source.Cached = true
			age := now.Sub(entry.storedAt).Seconds()
			response.Source.CacheAgeSeconds = &age
			return response, CacheHit, nil
		}
	}

	response, err := s.client.FetchProfile(ctx, validated)
	if err != nil {
		return Response{}, CacheMiss, err
	}
	response.Cached = false
	response.Source.Cached = false
	response.Source.CacheAgeSeconds = nil
	if s.ttl > 0 {
		s.mu.Lock()
		s.cache[validated] = cacheEntry{response: response, storedAt: now}
		s.mu.Unlock()
	}
	return response, CacheMiss, nil
}
