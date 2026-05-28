package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// MusicSource 跨插件 fan-out 返回的候选音源。
// Score 由相似度打分 + metrics 权重综合而来,Orchestrator 按 Score 降序逐个尝试。
type MusicSource struct {
	PluginEntryPath string
	SourceData      string // 原始 JSON 字符串(opaque,传给插件 music/url 用)
	Title           string
	Artist          string
	Album           string
	Duration        int
	CoverURL        string
	Score           float64
}

// ResolverOpts SourceResolver 的可调参数。
type ResolverOpts struct {
	PerPluginTimeout time.Duration // 单个插件 search 调用超时;默认 5s
	GlobalTimeout    time.Duration // 整个 fan-out 总超时;默认 8s
	MinScore         float64       // 候选最低分;默认 0.6
	MaxResults       int           // 最多返回多少候选;默认 5
	ExcludeRed       bool          // 是否过滤 HealthRed 插件;默认 true
	CacheTTL         time.Duration // LRU 缓存有效期;默认 5min
}

func DefaultResolverOpts() ResolverOpts {
	return ResolverOpts{
		PerPluginTimeout: 5 * time.Second,
		GlobalTimeout:    8 * time.Second,
		MinScore:         0.6,
		MaxResults:       5,
		ExcludeRed:       true,
		CacheTTL:         5 * time.Minute,
	}
}

// PluginLister 抽象 jsplugin.Manager.ListActive(),避免依赖具体类型。
// 返回的 PluginInfo 只要求有 EntryPath 字段(用接口的实现拿)。
type PluginLister interface {
	ListActiveEntryPaths() []string
}

// SourceResolver 跨插件搜索同名歌曲,返回排序后的候选音源。
//
// 实现要点:
//   - fan-out 并发调每个 active 音源插件的 /api/search(POST)
//   - 收到结果后用 (title/artist/duration) 相似度打分
//   - 用 SourceMetrics 的 WeightedScore 给可靠插件加权
//   - 短期 LRU 缓存避免重复 fan-out
type SourceResolver struct {
	pluginLister PluginLister
	invoker      PluginInvoker
	metrics      *SourceMetrics
	opts         ResolverOpts

	mu    sync.Mutex
	cache map[string]*resolverCacheEntry
}

type resolverCacheEntry struct {
	sources []MusicSource
	expiry  time.Time
}

func NewSourceResolver(lister PluginLister, invoker PluginInvoker, metrics *SourceMetrics, opts ResolverOpts) *SourceResolver {
	if opts.PerPluginTimeout == 0 {
		opts = DefaultResolverOpts()
	}
	return &SourceResolver{
		pluginLister: lister,
		invoker:      invoker,
		metrics:      metrics,
		opts:         opts,
		cache:        make(map[string]*resolverCacheEntry),
	}
}

// searchRequestBody 调用插件 /api/search 的请求体(POST)。
type searchRequestBody struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

// searchResponse 插件 /api/search 的响应。
type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Title      string          `json:"title"`
	Artist     string          `json:"artist"`
	Album      string          `json:"album,omitempty"`
	Duration   int             `json:"duration"`
	CoverURL   string          `json:"cover_url,omitempty"`
	SourceData json.RawMessage `json:"source_data"`
}

// Discover 跨插件搜索 song 的备选音源,排除指定的 entryPath(包含主源避免重复)。
//
// 返回时已按综合分数降序排列、过滤掉 < MinScore 的候选、并截取到 MaxResults。
// 失败的插件单独 timeout,不影响整体;所有插件都失败时返回空数组(不返回 error)。
func (r *SourceResolver) Discover(ctx context.Context, song *SongInfo, excludePlugins []string) ([]MusicSource, error) {
	if song == nil {
		return nil, nil
	}
	cacheKey := normalize(song.Title) + "|" + normalize(song.Artist)
	excludeSet := make(map[string]struct{}, len(excludePlugins))
	for _, p := range excludePlugins {
		excludeSet[p] = struct{}{}
	}

	// 检查缓存
	if cached := r.cacheGet(cacheKey); cached != nil {
		return filterExcluded(cached, excludeSet), nil
	}

	// 枚举可用插件
	entryPaths := r.pluginLister.ListActiveEntryPaths()
	candidates := make([]string, 0, len(entryPaths))
	for _, ep := range entryPaths {
		if _, skip := excludeSet[ep]; skip {
			continue
		}
		if r.opts.ExcludeRed && r.metrics != nil && r.metrics.Class(ep) == HealthRed {
			continue
		}
		candidates = append(candidates, ep)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// fan-out
	globalCtx, cancel := context.WithTimeout(ctx, r.opts.GlobalTimeout)
	defer cancel()

	type partial struct {
		entryPath string
		sources   []MusicSource
	}
	ch := make(chan partial, len(candidates))
	keyword := strings.TrimSpace(song.Title + " " + song.Artist)

	for _, ep := range candidates {
		go func(entryPath string) {
			subCtx, subCancel := context.WithTimeout(globalCtx, r.opts.PerPluginTimeout)
			defer subCancel()
			results := r.searchOne(subCtx, entryPath, keyword)
			scored := make([]MusicSource, 0, len(results))
			for _, sr := range results {
				if len(sr.SourceData) == 0 {
					continue
				}
				baseScore := similarityScore(song.Title, song.Artist, song.Duration,
					sr.Title, sr.Artist, sr.Duration)
				weighted := baseScore
				if r.metrics != nil {
					weighted = baseScore * r.metrics.WeightedScore(entryPath)
				}
				if weighted < r.opts.MinScore {
					continue
				}
				scored = append(scored, MusicSource{
					PluginEntryPath: entryPath,
					SourceData:      string(sr.SourceData),
					Title:           sr.Title,
					Artist:          sr.Artist,
					Album:           sr.Album,
					Duration:        sr.Duration,
					CoverURL:        sr.CoverURL,
					Score:           weighted,
				})
			}
			ch <- partial{entryPath: entryPath, sources: scored}
		}(ep)
	}

	all := make([]MusicSource, 0)
	for i := 0; i < len(candidates); i++ {
		select {
		case p := <-ch:
			all = append(all, p.sources...)
		case <-globalCtx.Done():
			i = len(candidates) // break
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > r.opts.MaxResults {
		all = all[:r.opts.MaxResults]
	}

	r.cachePut(cacheKey, all)
	return filterExcluded(all, excludeSet), nil
}

// searchOne 单个插件的 search 调用,失败时返回空切片(不向上传播错误)。
func (r *SourceResolver) searchOne(ctx context.Context, entryPath, keyword string) []searchResult {
	body, _ := json.Marshal(searchRequestBody{Keyword: keyword, Page: 1, PageSize: 20})
	status, _, respBody, err := r.invoker.InvokeHTTP(ctx, entryPath, http.MethodPost, "/api/search", url.Values{}, body)
	if err != nil || status != http.StatusOK {
		return nil
	}
	var resp searchResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil
	}
	return resp.Results
}

func (r *SourceResolver) cacheGet(key string) []MusicSource {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.cache[key]
	if !ok {
		return nil
	}
	if time.Now().After(e.expiry) {
		delete(r.cache, key)
		return nil
	}
	return e.sources
}

func (r *SourceResolver) cachePut(key string, sources []MusicSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = &resolverCacheEntry{
		sources: sources,
		expiry:  time.Now().Add(r.opts.CacheTTL),
	}
	// 顺便清掉过期项,避免缓存无限增长
	now := time.Now()
	for k, v := range r.cache {
		if now.After(v.expiry) {
			delete(r.cache, k)
		}
	}
}

func filterExcluded(sources []MusicSource, exclude map[string]struct{}) []MusicSource {
	if len(exclude) == 0 {
		return sources
	}
	out := make([]MusicSource, 0, len(sources))
	for _, s := range sources {
		if _, skip := exclude[s.PluginEntryPath]; !skip {
			out = append(out, s)
		}
	}
	return out
}

// normalize 标准化字符串用于缓存 key 与相似度比较:小写、去空格、去括号内容。
func normalize(s string) string {
	s = strings.ToLower(s)
	// 去括号内容:(...) [...] 【...】 (中文括号)
	for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"【", "】"}, {"(", ")"}} {
		for {
			start := strings.Index(s, pair[0])
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], pair[1])
			if end < 0 {
				break
			}
			s = s[:start] + s[start+end+len(pair[1]):]
		}
	}
	return strings.Join(strings.Fields(s), "")
}

// similarityScore 综合相似度:0.5*titleSim + 0.3*artistSim + 0.2*durationSim
// title/artist 用编辑距离归一化,duration 用相对差。
// d1 是 song.Duration(float64,秒),d2 是搜索结果的 duration(int,秒)。
func similarityScore(t1, a1 string, d1 float64, t2, a2 string, d2 int) float64 {
	titleSim := stringSimilarity(normalize(t1), normalize(t2))
	artistSim := artistSimilarity(a1, a2)
	durationSim := 0.5 // 默认中性
	if d1 > 0 && d2 > 0 {
		diff := d1 - float64(d2)
		if diff < 0 {
			diff = -diff
		}
		ratio := diff / d1
		if ratio > 1 {
			ratio = 1
		}
		durationSim = 1 - ratio
	}
	return 0.5*titleSim + 0.3*artistSim + 0.2*durationSim
}

// artistSimilarity 多艺术家按 "/"、"&"、"," 切分,取集合 IoU。
func artistSimilarity(a1, a2 string) float64 {
	set1 := splitArtists(a1)
	set2 := splitArtists(a2)
	if len(set1) == 0 || len(set2) == 0 {
		return 0.3 // 一方缺失时给个低分但不为 0
	}
	intersect := 0
	for k := range set1 {
		if _, ok := set2[k]; ok {
			intersect++
		}
	}
	union := len(set1) + len(set2) - intersect
	if union == 0 {
		return 0
	}
	return float64(intersect) / float64(union)
}

func splitArtists(s string) map[string]struct{} {
	out := make(map[string]struct{})
	s = strings.ReplaceAll(s, "&", "/")
	s = strings.ReplaceAll(s, ",", "/")
	s = strings.ReplaceAll(s, "、", "/")
	s = strings.ReplaceAll(s, " feat. ", "/")
	s = strings.ReplaceAll(s, " feat ", "/")
	for _, p := range strings.Split(s, "/") {
		p = normalize(p)
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}

// stringSimilarity 用 Levenshtein 距离归一到 [0, 1]。
func stringSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	d := levenshtein(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 1
	}
	return 1 - float64(d)/float64(maxLen)
}

// levenshtein 经典编辑距离(rune 安全),O(n*m) 时空复杂度。
// 对短字符串(歌名/艺术家)够用。
func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}
