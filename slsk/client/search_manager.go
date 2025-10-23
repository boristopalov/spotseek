package client

import (
	"fmt"
	"log/slog"
	"spotseek/slsk/fileshare"
	"sync"
	"time"
)

type SearchStatus string

const (
	SearchStatusActive    SearchStatus = "active"
	SearchStatusCompleted SearchStatus = "completed"
	SearchStatusExpired   SearchStatus = "expired"
)

type Search struct {
	ID        uint32                      `json:"id"`
	Query     string                      `json:"query"`
	Status    SearchStatus                `json:"status"`
	CreatedAt time.Time                   `json:"createdAt"`
	Results   []fileshare.SearchResult    `json:"results"`
	mu        sync.RWMutex
}

func (s *Search) AddResult(result fileshare.SearchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Results = append(s.Results, result)
}

func (s *Search) GetResults() []fileshare.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to avoid race conditions
	results := make([]fileshare.SearchResult, len(s.Results))
	copy(results, s.Results)
	return results
}

func (s *Search) GetResultCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Results)
}

func (s *Search) SetStatus(status SearchStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

func (s *Search) GetStatus() SearchStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

type SearchManager struct {
	searches map[uint32]*Search
	mu       sync.RWMutex
	ttl      time.Duration
	logger   *slog.Logger
}

func NewSearchManager(ttl time.Duration, logger *slog.Logger) *SearchManager {
	sm := &SearchManager{
		searches: make(map[uint32]*Search),
		ttl:      ttl,
		logger:   logger,
	}
	go sm.cleanupExpiredSearches()
	return sm
}

func (sm *SearchManager) CreateSearch(id uint32, query string) *Search {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	search := &Search{
		ID:        id,
		Query:     query,
		Status:    SearchStatusActive,
		CreatedAt: time.Now(),
		Results:   make([]fileshare.SearchResult, 0),
	}
	sm.searches[id] = search
	sm.logger.Info("Created search", "id", id, "query", query)
	return search
}

func (sm *SearchManager) GetSearch(id uint32) (*Search, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	search, exists := sm.searches[id]
	if !exists {
		return nil, fmt.Errorf("search not found: %d", id)
	}
	return search, nil
}

func (sm *SearchManager) AddResult(token uint32, result fileshare.SearchResult) error {
	sm.mu.RLock()
	search, exists := sm.searches[token]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("search not found: %d", token)
	}

	search.AddResult(result)
	return nil
}

func (sm *SearchManager) cleanupExpiredSearches() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, search := range sm.searches {
			if now.Sub(search.CreatedAt) > sm.ttl {
				search.SetStatus(SearchStatusExpired)
				delete(sm.searches, id)
				sm.logger.Info("Cleaned up expired search", "id", id, "query", search.Query)
			}
		}
		sm.mu.Unlock()
	}
}
