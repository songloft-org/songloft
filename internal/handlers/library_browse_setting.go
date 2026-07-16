package handlers

import (
	"encoding/json"
	"net/http"
	"slices"

	"songloft/internal/models"
)

const libraryBrowseKey = "library_browse_config"

// libraryBrowseView 曲库浏览的一个视图条目。
// Key 为视图标识（见 libraryViewKeys）；Visible 控制是否在曲库页展示。
type libraryBrowseView struct {
	Key     string `json:"key"`
	Visible bool   `json:"visible"`
}

// libraryBrowseSetting 曲库统一浏览页的视图显示 + 顺序配置。
type libraryBrowseSetting struct {
	Views []libraryBrowseView `json:"views"`
}

// libraryViewKeys 是 14 个合法视图 key 的**默认顺序**，按三组连续排列：
//   - 歌曲组：all(全部)/local(本地)/remote(网络)/radio(电台) —— 按 type 过滤的扁平歌曲列表；
//   - 分类组：artist/album/genre/year/decade/language/style —— facet 分类聚合，下钻歌曲；
//   - 歌单组：playlist(全部歌单)/playlist_normal(普通歌单)/playlist_radio(电台歌单) —— 歌单卡片列表。
// 前端渲染时按组固定顺序展示并在组间加分割线，组内顺序沿用用户配置。
var libraryViewKeys = []string{
	"all", "local", "remote", "radio",
	"artist", "album", "genre", "year", "decade", "language", "style",
	"playlist", "playlist_normal", "playlist_radio",
}

// defaultLibraryBrowse 默认全部可见、按 libraryViewKeys 顺序。
var defaultLibraryBrowse = libraryBrowseSetting{Views: defaultLibraryBrowseViews()}

func defaultLibraryBrowseViews() []libraryBrowseView {
	views := make([]libraryBrowseView, len(libraryViewKeys))
	for i, k := range libraryViewKeys {
		views[i] = libraryBrowseView{Key: k, Visible: true}
	}
	return views
}

func isValidLibraryViewKey(key string) bool {
	return slices.Contains(libraryViewKeys, key)
}

// GetLibraryBrowseSetting 获取曲库浏览视图配置
// @Summary 获取曲库浏览视图配置
// @Description 获取用户自定义的曲库统一浏览页视图显示与顺序。共 14 个视图，分三组：歌曲组 all(全部)/local(本地)/remote(网络)/radio(电台)；分类组 artist(歌手)/album(专辑)/genre(流派)/year(年份)/decade(年代)/language(语种)/style(风格)；歌单组 playlist(全部歌单)/playlist_normal(普通歌单)/playlist_radio(电台歌单)。未配置时返回默认（全部可见、默认顺序）。返回始终包含完整 14 项。
// @Tags 设置
// @Produce json
// @Success 200 {object} libraryBrowseSetting "曲库浏览视图配置"
// @Security BearerAuth
// @Router /settings/library-browse [get]
func (h *ConfigHandler) GetLibraryBrowseSetting(w http.ResponseWriter, r *http.Request) {
	var cfg libraryBrowseSetting
	if err := h.configService.GetJSON(libraryBrowseKey, &cfg); err != nil {
		respondJSON(w, http.StatusOK, defaultLibraryBrowse)
		return
	}
	respondJSON(w, http.StatusOK, normalizeLibraryBrowse(cfg))
}

// UpdateLibraryBrowseSetting 保存曲库浏览视图配置
// @Summary 保存曲库浏览视图配置
// @Description 保存用户自定义的曲库浏览页视图显示与顺序。每个 view 的 key 必须属于合法的 14 个 key 且不能重复；未出现的 key 会按默认顺序补到末尾（visible=true），保证返回完整 14 项。
// @Tags 设置
// @Accept json
// @Produce json
// @Param request body libraryBrowseSetting true "曲库浏览视图配置"
// @Success 200 {object} libraryBrowseSetting "保存后的配置"
// @Failure 400 {object} models.ErrorResponse "请求格式错误或校验失败"
// @Failure 500 {object} models.ErrorResponse "保存配置失败"
// @Security BearerAuth
// @Router /settings/library-browse [put]
func (h *ConfigHandler) UpdateLibraryBrowseSetting(w http.ResponseWriter, r *http.Request) {
	var req libraryBrowseSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}

	seen := make(map[string]bool, len(req.Views))
	for _, v := range req.Views {
		if !isValidLibraryViewKey(v.Key) {
			respondError(w, http.StatusBadRequest, "非法的视图 key: "+v.Key, nil)
			return
		}
		if seen[v.Key] {
			respondError(w, http.StatusBadRequest, "视图 key 不能重复: "+v.Key, nil)
			return
		}
		seen[v.Key] = true
	}

	normalized := normalizeLibraryBrowse(req)
	if err := h.configService.SetJSON(libraryBrowseKey, normalized); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, normalized)
}

// normalizeLibraryBrowse 去掉非法 key，并把缺失的 key 按默认顺序补到末尾（visible=true），
// 保证返回始终是完整、无重复的 11 项。
func normalizeLibraryBrowse(cfg libraryBrowseSetting) libraryBrowseSetting {
	seen := make(map[string]bool, len(cfg.Views))
	views := make([]libraryBrowseView, 0, len(libraryViewKeys))
	for _, v := range cfg.Views {
		if !isValidLibraryViewKey(v.Key) || seen[v.Key] {
			continue
		}
		seen[v.Key] = true
		views = append(views, v)
	}
	for _, k := range libraryViewKeys {
		if !seen[k] {
			views = append(views, libraryBrowseView{Key: k, Visible: true})
		}
	}
	return libraryBrowseSetting{Views: views}
}

// Ensure models.ErrorResponse is referenced for swagger generation.
var _ = models.ErrorResponse{}
