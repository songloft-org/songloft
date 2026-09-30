package source

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"testing"
)

// fakePluginLister 返回固定的 active 插件列表。
type fakePluginLister struct {
	paths []string
}

func (f fakePluginLister) ListActiveEntryPaths() []string { return f.paths }

// fakePluginInvoker 对每个插件返回预置的搜索结果。
type fakePluginInvoker struct {
	results map[string][]searchResult
	calls   map[string]int
}

func (f *fakePluginInvoker) InvokeHTTP(
	_ context.Context,
	entryPath, _, _ string,
	_ interface{},
	_ []byte,
) (int, map[string]string, []byte, error) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[entryPath]++
	body, _ := json.Marshal(searchResponse{Results: f.results[entryPath]})
	return http.StatusOK, nil, body, nil
}

func matchingResult(id string, duration int) searchResult {
	return searchResult{
		Title:      "Song",
		Artist:     "Artist",
		Duration:   duration,
		SourceData: json.RawMessage(`{"id":"` + id + `"}`),
	}
}

// 同 title/artist 的两首歌，主源插件不同：缓存键必须包含排除集，
// 否则第二首歌会命中第一首的缓存（其中不含第二首的主源），导致可用音源被漏掉。
func TestDiscover_CacheKeyMustIncludeExcludedPlugins(t *testing.T) {
	lister := fakePluginLister{paths: []string{"p1", "p2"}}
	inv := &fakePluginInvoker{results: map[string][]searchResult{
		"p1": {matchingResult("1", 200)},
		"p2": {matchingResult("2", 200)},
	}}
	r := NewSourceResolver(lister, inv, nil, DefaultResolverOpts())
	song := &SongInfo{Title: "Song", Artist: "Artist", Duration: 200, PluginEntryPath: "p1"}

	// 第一首：主源 p1 → 排除 p1，应只返回 p2，并写入缓存。
	first, err := r.Discover(context.Background(), song, []string{"p1"})
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	if len(first) != 1 || first[0].PluginEntryPath != "p2" {
		t.Fatalf("first call: want [p2], got %+v", first)
	}

	// 第二首：同 title/artist 但主源是 p2 → 排除 p2，应返回 p1。
	// 缓存键若不含排除集，这里会命中第一首的缓存并在过滤后返回空。
	second, err := r.Discover(context.Background(), song, []string{"p2"})
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	if len(second) != 1 || second[0].PluginEntryPath != "p1" {
		t.Fatalf("second call: want [p1], got %+v", second)
	}
}

// 候选得分依赖 song.Duration（时长越接近分越高），因此缓存键也必须包含 duration，
// 否则同 title/artist、不同时长的歌会复用为别的时长算出的得分。
func TestDiscover_CacheKeyMustIncludeDuration(t *testing.T) {
	lister := fakePluginLister{paths: []string{"p1"}}
	inv := &fakePluginInvoker{results: map[string][]searchResult{
		"p1": {matchingResult("1", 200)},
	}}
	r := NewSourceResolver(lister, inv, nil, DefaultResolverOpts())

	long := &SongInfo{Title: "Song", Artist: "Artist", Duration: 200, PluginEntryPath: "p0"}
	got, err := r.Discover(context.Background(), long, nil)
	if err != nil {
		t.Fatalf("long Discover: %v", err)
	}
	if len(got) != 1 || math.Abs(got[0].Score-1.0) > 1e-9 {
		t.Fatalf("duration matching: want score 1.0, got %+v", got)
	}

	// 时长差 180s / 20s → durationSim 归零，得分应为 0.5*1 + 0.3*1 + 0.2*0 = 0.8。
	short := &SongInfo{Title: "Song", Artist: "Artist", Duration: 20, PluginEntryPath: "p0"}
	got, err = r.Discover(context.Background(), short, nil)
	if err != nil {
		t.Fatalf("short Discover: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("duration mismatch: want 1 candidate, got %+v", got)
	}
	if math.Abs(got[0].Score-0.8) > 1e-9 {
		t.Fatalf("duration mismatch: want score 0.8, got %v", got[0].Score)
	}
}
