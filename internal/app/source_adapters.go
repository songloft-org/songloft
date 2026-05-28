package app

import (
	"context"

	"mimusic/internal/jsplugin"
	"mimusic/internal/services"
	"mimusic/internal/services/source"
)

// 本文件集中 source 子系统所需的接口适配器。
// 通过 adapter 让 services.MetadataExtractor / jsplugin.Manager / services.SongService
// 分别满足 source.Prober / PluginInvoker / PluginLister / SongUpdater 接口,
// 避免下层包反向依赖 services 或 jsplugin。

// proberAdapter 让 *services.MetadataExtractor 满足 source.Prober
type proberAdapter struct {
	m *services.MetadataExtractor
}

func (a *proberAdapter) ProbeForValidation(ctx context.Context, filePath string) (source.AudioInfoLike, error) {
	info, err := a.m.ProbeForValidation(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// jsPluginInvokerAdapter 让 *jsplugin.Manager 满足 source.PluginInvoker
type jsPluginInvokerAdapter struct {
	m *jsplugin.Manager
}

func (a *jsPluginInvokerAdapter) InvokeHTTP(
	ctx context.Context,
	entryPath, method, path string,
	query interface{},
	body []byte,
) (statusCode int, respHeaders map[string]string, respBody []byte, err error) {
	return a.m.InvokeHTTP(ctx, entryPath, method, path, query, body)
}

// jsPluginListerAdapter 让 *jsplugin.Manager 满足 source.PluginLister
type jsPluginListerAdapter struct {
	m *jsplugin.Manager
}

func (a *jsPluginListerAdapter) ListActiveEntryPaths() []string {
	plugins := a.m.ListActive()
	out := make([]string, 0, len(plugins))
	for _, p := range plugins {
		if p == nil || p.EntryPath == "" {
			continue
		}
		out = append(out, p.EntryPath)
	}
	return out
}

// songUpdaterAdapter 让 *services.SongService 满足 source.SongUpdater
type songUpdaterAdapter struct {
	s *services.SongService
}

func (a *songUpdaterAdapter) UpdateSongSource(ctx context.Context, id int64, pluginEntryPath, sourceData string) error {
	return a.s.UpdateSongSource(ctx, id, pluginEntryPath, sourceData)
}

func (a *songUpdaterAdapter) UpdateSongDuration(ctx context.Context, id int64, duration float64) error {
	return a.s.UpdateSongDuration(ctx, id, duration)
}

// reassignAdapter 包装 source.SourceOrchestrator + services.SongService,
// 给 cache handler 提供一个简单的 AsyncReassign(songID) 接口。
// 把"按 id 加载 song"这个职责从 source 包剥离到这里。
type reassignAdapter struct {
	orch *source.SourceOrchestrator
	s    *services.SongService
}

func (a *reassignAdapter) AsyncReassign(songID int64) {
	a.orch.AsyncReassign(songID, func(ctx context.Context, id int64) (*source.SongInfo, error) {
		song, err := a.s.GetByID(ctx, id)
		if err != nil || song == nil {
			return nil, err
		}
		return &source.SongInfo{
			ID:              song.ID,
			Title:           song.Title,
			Artist:          song.Artist,
			Album:           song.Album,
			Duration:        song.Duration,
			PluginEntryPath: song.PluginEntryPath,
			SourceData:      song.SourceData,
		}, nil
	})
}
