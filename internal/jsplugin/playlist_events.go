package jsplugin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"songloft/internal/models"
)

type playlistChangeBatch struct {
	changes []models.PlaylistSongsChangeEvent
	targets []string
}

func (m *Manager) RegisterPlaylistSongsChange(entryPath string) {
	m.playlistSongsSubscribers.Store(entryPath, true)
}

func (m *Manager) UnregisterPlaylistSongsChange(entryPath string) {
	m.playlistSongsSubscribers.Delete(entryPath)
}

// BroadcastPlaylistSongsChange queues committed changes without waiting for JS or
// lazy loading. One worker preserves submission order, including add/remove pairs.
// Delivery is best effort, with the same bounded queues as other plugin events.
func (m *Manager) BroadcastPlaylistSongsChange(changes []models.PlaylistSongsChangeEvent) {
	if len(changes) == 0 {
		return
	}
	var targets []string
	m.playlistSongsSubscribers.Range(func(key, _ any) bool {
		targets = append(targets, key.(string))
		return true
	})
	if len(targets) == 0 {
		return
	}
	batch := playlistChangeBatch{changes: slices.Clone(changes), targets: targets}
	select {
	case <-m.playlistEventsDone:
	case m.playlistEvents <- batch:
	default:
		slog.Warn("playlist change event queue full", "changes", len(changes))
	}
}

func (m *Manager) startPlaylistEvents() {
	ctx, cancel := context.WithCancel(context.Background())
	m.playlistEventsCancel = cancel
	m.playlistEvents = make(chan playlistChangeBatch, defaultQueueSize)
	m.playlistEventsDone = make(chan struct{})
	go func() {
		defer close(m.playlistEventsDone)
		for {
			select {
			case <-ctx.Done():
				return
			case batch := <-m.playlistEvents:
				m.deliverPlaylistChanges(ctx, batch)
			}
		}
	}()
}

func (m *Manager) deliverPlaylistChanges(ctx context.Context, batch playlistChangeBatch) {
	for _, entryPath := range batch.targets {
		if ctx.Err() != nil {
			return
		}
		if _, ok := m.playlistSongsSubscribers.Load(entryPath); !ok {
			continue
		}
		loadCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		_, err := m.EnsureLoaded(loadCtx, entryPath)
		cancel()
		if err != nil {
			if errors.Is(err, ErrPluginDisabled) || errors.Is(err, ErrPluginNotFound) {
				m.UnregisterPlaylistSongsChange(entryPath)
			}
			slog.Debug("playlist change: lazy load failed", "plugin", entryPath, "error", err)
			continue
		}
		// onInit may have declined to restore a persisted subscription.
		if _, ok := m.playlistSongsSubscribers.Load(entryPath); !ok {
			continue
		}
		// One message per committed operation: a batch larger than the actor's
		// queue must not overflow it solely because it contains many members.
		if err := m.scheduler.Send(&Message{Type: MsgPlaylistSongsChange, Target: entryPath, Data: batch.changes, Ctx: ctx}); err != nil {
			slog.Warn("playlist change: send failed", "plugin", entryPath, "error", err)
		}
	}
}

func (s *JSService) handlePlaylistSongsChange(msg *Message) *Message {
	changes, ok := msg.Data.([]models.PlaylistSongsChangeEvent)
	if !ok {
		return nil
	}
	for _, change := range changes {
		if msg.Ctx != nil && msg.Ctx.Err() != nil {
			break
		}
		data, err := json.Marshal(change)
		if err == nil {
			_, err = s.jsManager.ExecuteJSCall(context.Background(), s.envID, "__dispatchPlaylistSongsChange", 5000, string(data))
		}
		if err != nil {
			slog.Warn("onPlaylistSongsChange failed", "plugin", s.plugin.EntryPath, "error", err)
		}
	}
	return nil
}
