package services

import (
	"context"
	"crypto/rand"
	"errors"
	"strconv"
	"sync"
	"time"
)

// FavoriteChangesResponse invalidates both built-in favorite caches. The cursor
// is opaque and changes across server restarts; clients must not order it.
type FavoriteChangesResponse struct {
	Cursor  string `json:"cursor"`
	Changed bool   `json:"changed"`
}

type favoriteChanges struct {
	mu       sync.Mutex
	epoch    string
	revision uint64
	changed  chan struct{}
	closed   bool
}

var ErrFavoriteChangesClosed = errors.New("favorite change subscription closed")

func newFavoriteChanges() *favoriteChanges {
	return &favoriteChanges{epoch: rand.Text(), changed: make(chan struct{})}
}

func (c *favoriteChanges) cursor() string {
	return c.epoch + ":" + strconv.FormatUint(c.revision, 10)
}

func (c *favoriteChanges) notify() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.revision++
	close(c.changed)
	c.changed = make(chan struct{})
}

func (c *favoriteChanges) wait(ctx context.Context, cursor string, timeout time.Duration) (FavoriteChangesResponse, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return FavoriteChangesResponse{}, ErrFavoriteChangesClosed
	}
	current, signal := c.cursor(), c.changed
	c.mu.Unlock()
	if current != cursor {
		return FavoriteChangesResponse{Cursor: current, Changed: true}, nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return FavoriteChangesResponse{}, ctx.Err()
	case <-signal:
	case <-timer.C:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return FavoriteChangesResponse{}, ErrFavoriteChangesClosed
	}
	current = c.cursor()
	return FavoriteChangesResponse{Cursor: current, Changed: current != cursor}, nil
}

// FavoriteChanges waits for a post-commit change to Favorites or Radio Favorites.
// A missing/stale cursor returns immediately so reconnects reconcile current data.
func (s *PlaylistService) FavoriteChanges(ctx context.Context, cursor string) (FavoriteChangesResponse, error) {
	return s.favoriteChanges.wait(ctx, cursor, 25*time.Second)
}

// CloseFavoriteChanges releases hanging requests before HTTP shutdown.
func (s *PlaylistService) CloseFavoriteChanges() {
	if s.favoriteChanges == nil {
		return
	}
	c := s.favoriteChanges
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.changed)
	}
}
