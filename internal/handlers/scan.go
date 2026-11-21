package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"songloft/internal/models"
	"songloft/internal/services"
)

// ScanHandler 扫描处理器
//
// 除扫描动作外，还承载扫描相关业务设置端点（/settings/music-path、
// /settings/scan-playlist-mode 等），把扫描相关 config key 的"业务化"读写收敛在此。
type ScanHandler struct {
	songService        *services.SongService
	scanner            *services.Scanner
	configService      *services.ConfigService
	fingerprintService *services.FingerprintService
	onMusicPathChanged func()                        // PUT /settings/music-path 完成后触发，重建 Scanner 等副作用
	onAutoScanChanged  func(services.AutoScanConfig) // PUT /settings/auto-scan 完成后触发，重启自动扫描调度
}

// NewScanHandler 创建扫描处理器。configService 可为 nil（仅纯扫描端点的测试场景）。
func NewScanHandler(songService *services.SongService, scanner *services.Scanner, configService *services.ConfigService) *ScanHandler {
	return &ScanHandler{
		songService:   songService,
		scanner:       scanner,
		configService: configService,
	}
}

// SetFingerprintService 注入指纹服务。
func (h *ScanHandler) SetFingerprintService(fs *services.FingerprintService) {
	h.fingerprintService = fs
}

// SetScanner 更新扫描器引用（配置变更时调用）
func (h *ScanHandler) SetScanner(scanner *services.Scanner) {
	h.scanner = scanner
}

// SetOnMusicPathChanged 注入 music_path 写后回调（重建 Scanner + 清排除目录中的歌曲）。
// 同一回调也注册到通用 /configs/{key} 的 onConfigChanged，让 admin 工具直改 music_path 时
// 副作用同样生效（保持两条入口语义对齐）。
func (h *ScanHandler) SetOnMusicPathChanged(cb func()) {
	h.onMusicPathChanged = cb
}

// SetOnAutoScanChanged 注入 auto_scan 写后回调（重启自动扫描调度器）。
func (h *ScanHandler) SetOnAutoScanChanged(cb func(services.AutoScanConfig)) {
	h.onAutoScanChanged = cb
}

// ScanRequest 扫描请求参数
type ScanRequest struct {
	Reimport               bool `json:"reimport"`
	ResetMetadataOverrides bool `json:"reset_metadata_overrides,omitempty"`
	// Paths 为目录级定向扫描（Issue #262）：为空时扫描整个音乐根目录（默认行为）；
	// 非空时只扫描给定目录（含子目录），过期记录清理也仅收敛到这些目录之内。
	// 每个目录必须位于音乐根目录之下，否则返回 400。
	Paths []string `json:"paths,omitempty"`
}

type ScanStartResponse struct {
	Message             string                       `json:"message"`
	ExcludedDirectories []services.ExcludedDirectory `json:"excluded_directories"`
}

// ScanAndImport 扫描并导入本地音乐（异步）
// @Summary 扫描并导入本地音乐
// @Description 异步扫描音乐目录并导入新发现的音乐文件到数据库，立即返回，可通过进度接口查询状态。
// @Description reimport=true 时对已入库文件也重新提取元数据；默认 false 走增量（跳过已存在且时长有效的文件）。
// @Description reset_metadata_overrides=true 仅可与 reimport=true 一起使用。成功读取时在扫描范围内用文件元数据替换手动字段。
// @Description paths 为目录级定向扫描（Issue #262）：省略/为空时扫描整个音乐根目录；非空时只扫描给定目录（含子目录），
// @Description 且过期记录清理仅收敛到这些目录之内（不影响其余曲库）。每个目录必须位于音乐根目录之下，否则返回 400。
// @Description 始终遵守排除规则。所有扫描起点均被排除时返回 400，不启动任务；部分被排除时在响应和进度中列出原因。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body ScanRequest false "扫描请求参数"
// @Success 200 {object} ScanStartResponse "扫描任务已启动，含被排除的扫描起点"
// @Failure 400 {object} map[string]string "指定目录不在音乐目录下，或所有扫描起点均被排除"
// @Failure 409 {object} map[string]string "扫描正在进行中"
// @Failure 500 {object} map[string]string "启动扫描失败"
// @Security BearerAuth
// @Router /scan [post]
func (h *ScanHandler) ScanAndImport(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var req ScanRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "无效的请求参数", err)
			return
		}
	}

	if req.ResetMetadataOverrides && !req.Reimport {
		respondError(w, http.StatusBadRequest, "重置手动元数据必须同时启用重新导入", nil)
		return
	}

	// 目录级定向扫描：校验每个目录都在音乐根目录之下，并去重、剔除互为父子的冗余目录。
	scopePaths, err := h.sanitizeScanPaths(req.Paths)
	if err != nil {
		respondError(w, http.StatusBadRequest, "指定目录不在音乐目录下", err)
		return
	}
	validation := h.scanner.ValidateRoots(scopePaths)
	if len(validation.Paths) == 0 {
		details := make([]string, 0, len(validation.ExcludedDirectories))
		for _, dir := range validation.ExcludedDirectories {
			details = append(details, fmt.Sprintf("%s (%s: %s)", dir.Path, dir.RuleType, dir.Rule))
		}
		respondError(w, http.StatusBadRequest, "所有扫描目录均被排除，请调整排除设置", fmt.Errorf("%s", strings.Join(details, "; ")))
		return
	}

	if err := h.songService.ScanAndImportWithMetadataReset(req.Reimport, scopePaths, req.ResetMetadataOverrides); err != nil {
		respondError(w, http.StatusConflict, "扫描正在进行中", err)
		return
	}

	respondJSON(w, http.StatusOK, ScanStartResponse{
		Message:             "扫描任务已启动",
		ExcludedDirectories: validation.ExcludedDirectories,
	})
}

// ValidateScan 检查所选扫描目录的排除规则。
// @Summary 检查扫描目录
// @Description 使用与扫描相同的路径校验和排除规则，返回可扫描起点及被排除起点的路径、规则类型（path/name）和规则值。不遍历子目录、不启动扫描、不修改配置。paths 省略或为空时检查音乐根目录。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body ScanRequest false "仅使用 paths 字段"
// @Success 200 {object} services.ScanValidation "可扫描目录及排除原因；paths 为空表示全部被排除"
// @Failure 400 {object} map[string]string "无效参数或目录不在音乐目录下"
// @Security BearerAuth
// @Router /scan/validate [post]
func (h *ScanHandler) ValidateScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "无效的请求参数", err)
			return
		}
	}
	paths, err := h.sanitizeScanPaths(req.Paths)
	if err != nil {
		respondError(w, http.StatusBadRequest, "指定目录不在音乐目录下", err)
		return
	}
	respondJSON(w, http.StatusOK, h.scanner.ValidateRoots(paths))
}

// sanitizeScanPaths 校验并规整定向扫描的目录列表：
//   - 每个路径 filepath.Clean 后必须等于音乐根目录或位于其下（防目录遍历）；
//   - 去重，并剔除已被列表中某个祖先目录覆盖的冗余子目录。
//
// 返回的切片为空表示全库扫描（调用方据此退化为默认行为）。
func (h *ScanHandler) sanitizeScanPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	root := filepath.Clean(h.scanner.GetMusicPath())
	seen := make(map[string]struct{}, len(paths))
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		cp := filepath.Clean(p)
		if !h.inMusicRoot(cp, root) {
			return nil, fmt.Errorf("路径必须在音乐目录下: %s", p)
		}
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		cleaned = append(cleaned, cp)
	}

	// 客户端明确传了 paths（意图定向扫描）但全部为空白 → 拒绝，
	// 避免静默退化为"全库扫描 + 全库过期清理"这种危险的意外行为。
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("未提供有效的目录")
	}

	// 剔除被列表中某个祖先目录覆盖的子目录（如同时选了 /m 与 /m/a，只保留 /m）。
	result := make([]string, 0, len(cleaned))
	for _, p := range cleaned {
		covered := false
		for _, other := range cleaned {
			if other != p && strings.HasPrefix(p, other+string(filepath.Separator)) {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, p)
		}
	}
	return result, nil
}

// inMusicRoot 判断 cleanPath（已 filepath.Clean）是否等于音乐根目录或位于其下。
// cleanRoot 需为已 filepath.Clean 的音乐根目录。用于防目录遍历攻击。
func (h *ScanHandler) inMusicRoot(cleanPath, cleanRoot string) bool {
	return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator))
}

// GetScanProgress 获取扫描进度
// @Summary 获取扫描进度
// @Description 获取当前扫描任务的进度信息
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Success 200 {object} services.ScanProgress "扫描进度信息"
// @Security BearerAuth
// @Router /scan/progress [get]
func (h *ScanHandler) GetScanProgress(w http.ResponseWriter, r *http.Request) {
	progress := h.songService.GetScanProgress()
	respondJSON(w, http.StatusOK, progress)
}

// CancelScan 取消扫描
// @Summary 取消扫描
// @Description 取消正在进行的扫描任务
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "取消成功"
// @Failure 400 {object} map[string]string "没有正在进行的扫描任务"
// @Security BearerAuth
// @Router /scan/cancel [post]
func (h *ScanHandler) CancelScan(w http.ResponseWriter, r *http.Request) {
	if !h.songService.CancelScan() {
		respondError(w, http.StatusBadRequest, "没有正在进行的扫描任务", nil)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "扫描任务已取消",
	})
}

// ListDirectories 获取子目录列表（目录树懒加载）
// @Summary 获取子目录列表
// @Description 返回指定路径下的一级子目录列表，用于目录树懒加载。path 为空时返回音乐根目录下的子目录
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param path query string false "目录路径（为空时使用音乐根目录）"
// @Success 200 {object} map[string]interface{} "子目录列表"
// @Failure 400 {object} map[string]string "无效的路径"
// @Failure 500 {object} map[string]string "读取目录失败"
// @Security BearerAuth
// @Router /scan/directories [get]
func (h *ScanHandler) ListDirectories(w http.ResponseWriter, r *http.Request) {
	requestPath := r.URL.Query().Get("path")
	musicRoot := h.scanner.GetMusicPath()

	// 如果未指定路径，使用音乐根目录
	targetPath := musicRoot
	if requestPath != "" {
		targetPath = requestPath
	}

	// 安全校验：确保请求路径在音乐根目录下，防止目录遍历攻击
	cleanTarget := filepath.Clean(targetPath)
	cleanRoot := filepath.Clean(musicRoot)
	if !h.inMusicRoot(cleanTarget, cleanRoot) {
		respondError(w, http.StatusBadRequest, "路径必须在音乐目录下", nil)
		return
	}

	dirs, err := h.scanner.ListSubDirs(targetPath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "读取目录失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"directories": dirs,
		"root":        musicRoot,
	})
}

// ListDirNames 获取所有目录名称（自动补全用）
// @Summary 获取所有目录名称
// @Description 递归收集音乐目录下所有唯一的目录名称，按字母排序返回，用于排除目录名称的自动补全。
// @Description 结果带 60 秒进程内缓存；万首级曲库首次遍历可能较慢，客户端超时后重试会直接命中缓存。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "目录名称列表"
// @Failure 500 {object} map[string]string "收集目录名称失败"
// @Security BearerAuth
// @Router /scan/dir-names [get]
func (h *ScanHandler) ListDirNames(w http.ResponseWriter, r *http.Request) {
	// 刻意脱开请求的取消信号：大曲库遍历可能超过客户端超时，若跟着 r.Context() 一起
	// 取消，遍历半途中止、缓存永远填不上，前端每次重试都从零再走一遍并再次 500
	// （songloft-org/songloft#447）。脱开后首次遍历会跑完并填缓存，重试立刻命中。
	names, err := h.scanner.CollectAllDirNames(context.WithoutCancel(r.Context()))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "收集目录名称失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"names": names,
	})
}

// ============================================================
// 业务化配置端点
// ============================================================
//
// 与 /api/v1/configs/{key} 的关系：
//   - /configs/{key} 是通用 KV，保留为 admin 入口（config_manager 编辑器）。
//   - 客户端业务功能一律走下方业务端点：强类型、自带默认值、PUT 后内联触发副作用。
//   - 详见 AGENTS.md「配置接口规范」章节。

const (
	autoScanConfigKey                = "auto_scan"
	musicPathConfigKey               = "music_path"
	scanPlaylistModeConfigKey        = "scan_playlist_mode"
	scanAutoCreatePlaylistsConfigKey = "scan_auto_create_playlists"
	scanTitleSourceConfigKey         = "scan_title_source"
	scanAutoFingerprintConfigKey     = "scan_auto_fingerprint"
)

// MusicPathSetting /settings/music-path 的请求与响应体。
// 与 config 表 music_path 行的 JSON value 结构完全一致，便于 admin 工具与业务端点互通。
type MusicPathSetting struct {
	Path                  string   `json:"path"`
	ExcludeDirs           []string `json:"exclude_dirs"`
	ExcludePaths          []string `json:"exclude_paths"`
	AutoCreateExcludeDirs []string `json:"auto_create_exclude_dirs"`
}

// GetMusicPathSetting GET /api/v1/settings/music-path
// @Summary 获取音乐路径与扫描排除配置
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} MusicPathSetting
// @Security BearerAuth
// @Router /settings/music-path [get]
func (h *ScanHandler) GetMusicPathSetting(w http.ResponseWriter, r *http.Request) {
	cfg := MusicPathSetting{
		Path:                  "music",
		ExcludeDirs:           []string{"@eaDir", "tmp"},
		ExcludePaths:          []string{},
		AutoCreateExcludeDirs: []string{"downloads"},
	}
	if h.configService != nil {
		// 未命中时保留默认值；命中则覆盖
		_ = h.configService.GetJSON(musicPathConfigKey, &cfg)
	}
	if cfg.ExcludeDirs == nil {
		cfg.ExcludeDirs = []string{}
	}
	if cfg.ExcludePaths == nil {
		cfg.ExcludePaths = []string{}
	}
	respondJSON(w, http.StatusOK, cfg)
}

// UpdateMusicPathSetting PUT /api/v1/settings/music-path
// @Summary 更新音乐路径与扫描排除配置
// @Description 写入 music_path 配置并触发 Scanner 重建 + 清理排除目录中的歌曲（与 admin /configs PUT 的副作用一致）。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body MusicPathSetting true "配置内容"
// @Success 200 {object} MusicPathSetting
// @Failure 400 {object} map[string]string "请求格式错误"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/music-path [put]
func (h *ScanHandler) UpdateMusicPathSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req MusicPathSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		respondError(w, http.StatusBadRequest, "path 不能为空", nil)
		return
	}
	if req.ExcludeDirs == nil {
		req.ExcludeDirs = []string{}
	}
	if req.ExcludePaths == nil {
		req.ExcludePaths = []string{}
	}
	if req.AutoCreateExcludeDirs == nil {
		req.AutoCreateExcludeDirs = []string{}
	}
	if err := h.configService.SetJSON(musicPathConfigKey, req); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	// 副作用与通用 PUT /configs/music_path 完全一致（onConfigChanged 回调），异步触发不阻塞 PUT 响应。
	if h.onMusicPathChanged != nil {
		go h.onMusicPathChanged()
	}
	respondJSON(w, http.StatusOK, req)
}

// scanPlaylistModeRequest /settings/scan-playlist-mode PUT 请求体
type scanPlaylistModeRequest struct {
	Mode string `json:"mode" example:"directory" enums:"directory,top_level,bubble_up"`
}

// scanPlaylistModeResponse /settings/scan-playlist-mode 响应体
type scanPlaylistModeResponse struct {
	Mode string `json:"mode" example:"directory"`
}

// GetPlaylistModeSetting GET /api/v1/settings/scan-playlist-mode
// @Summary 获取歌单创建方式
// @Description 返回扫描后自动创建歌单的目录归并模式。directory：每个文件夹生成独立歌单；top_level：按一级子目录合并歌单；bubble_up：歌曲同时出现在所有上级文件夹歌单。默认 directory。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} scanPlaylistModeResponse "返回 mode 字段"
// @Security BearerAuth
// @Router /settings/scan-playlist-mode [get]
func (h *ScanHandler) GetPlaylistModeSetting(w http.ResponseWriter, r *http.Request) {
	mode := models.PlaylistModeDirectory
	if h.configService != nil {
		mode = h.configService.GetString(scanPlaylistModeConfigKey, models.PlaylistModeDirectory)
	}
	respondJSON(w, http.StatusOK, scanPlaylistModeResponse{Mode: mode})
}

// UpdatePlaylistModeSetting PUT /api/v1/settings/scan-playlist-mode
// @Summary 更新歌单创建方式
// @Description 设置扫描后自动创建歌单的目录归并模式。directory：每个文件夹生成独立歌单；top_level：按一级子目录合并歌单；bubble_up：歌曲同时出现在所有上级文件夹歌单。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body scanPlaylistModeRequest true "模式请求"
// @Success 200 {object} scanPlaylistModeResponse "返回 mode 字段"
// @Failure 400 {object} map[string]string "请求格式错误或 mode 值非法"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/scan-playlist-mode [put]
func (h *ScanHandler) UpdatePlaylistModeSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req scanPlaylistModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	switch req.Mode {
	case models.PlaylistModeDirectory, models.PlaylistModeTopLevel, models.PlaylistModeBubbleUp:
	default:
		respondError(w, http.StatusBadRequest, "mode 值非法，可选：directory / top_level / bubble_up", nil)
		return
	}
	if err := h.configService.Set(scanPlaylistModeConfigKey, req.Mode); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, scanPlaylistModeResponse{Mode: req.Mode})
}

// scanAutoCreatePlaylistsRequest /settings/scan-auto-create-playlists PUT 请求体
type scanAutoCreatePlaylistsRequest struct {
	Enabled bool `json:"enabled"`
}

// GetAutoCreatePlaylistsSetting GET /api/v1/settings/scan-auto-create-playlists
// @Summary 获取「扫描后自动创建歌单」开关
// @Description 控制扫描完成后是否根据音乐目录结构自动创建歌单。默认启用（true）。关闭后扫描仅入库歌曲，不再自动建歌单。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} map[string]bool "返回 enabled 字段"
// @Security BearerAuth
// @Router /settings/scan-auto-create-playlists [get]
func (h *ScanHandler) GetAutoCreatePlaylistsSetting(w http.ResponseWriter, r *http.Request) {
	enabled := true
	if h.configService != nil {
		enabled = h.configService.GetBool(scanAutoCreatePlaylistsConfigKey, true)
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// UpdateAutoCreatePlaylistsSetting PUT /api/v1/settings/scan-auto-create-playlists
// @Summary 更新「扫描后自动创建歌单」开关
// @Description 控制扫描完成后是否根据音乐目录结构自动创建歌单。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body scanAutoCreatePlaylistsRequest true "开关请求"
// @Success 200 {object} map[string]bool "返回 enabled 字段"
// @Failure 400 {object} map[string]string "请求格式错误"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/scan-auto-create-playlists [put]
func (h *ScanHandler) UpdateAutoCreatePlaylistsSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req scanAutoCreatePlaylistsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	val := "false"
	if req.Enabled {
		val = "true"
	}
	if err := h.configService.Set(scanAutoCreatePlaylistsConfigKey, val); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}

// scanAutoFingerprintRequest /settings/scan-auto-fingerprint PUT 请求体
type scanAutoFingerprintRequest struct {
	Enabled bool `json:"enabled"`
}

// GetScanAutoFingerprintSetting GET /api/v1/settings/scan-auto-fingerprint
// @Summary 获取「扫描后自动计算音频指纹」开关
// @Description 控制扫描完成后是否自动为缺失指纹的本地歌曲计算 chromaprint 音频指纹。默认关闭（false）：指纹只服务于「重复歌曲检测」和插件歌词/封面搜索，属按需功能，全库自动计算会长时间占用 CPU。关闭时可在重复检测页手动触发 POST /scan/fingerprints。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} map[string]bool "返回 enabled 字段"
// @Security BearerAuth
// @Router /settings/scan-auto-fingerprint [get]
func (h *ScanHandler) GetScanAutoFingerprintSetting(w http.ResponseWriter, r *http.Request) {
	enabled := false
	if h.configService != nil {
		enabled = h.configService.GetBool(scanAutoFingerprintConfigKey, false)
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// UpdateScanAutoFingerprintSetting PUT /api/v1/settings/scan-auto-fingerprint
// @Summary 更新「扫描后自动计算音频指纹」开关
// @Description 开启后每次扫描结束会在后台为缺失指纹的本地歌曲计算 chromaprint 指纹（并发按 CPU 自适应，单文件采样前 120 秒，失败只尝试一次）。大音乐库开启前请留意 CPU 开销。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body scanAutoFingerprintRequest true "开关请求"
// @Success 200 {object} map[string]bool "返回 enabled 字段"
// @Failure 400 {object} map[string]string "请求格式错误"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/scan-auto-fingerprint [put]
func (h *ScanHandler) UpdateScanAutoFingerprintSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req scanAutoFingerprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	val := "false"
	if req.Enabled {
		val = "true"
	}
	if err := h.configService.Set(scanAutoFingerprintConfigKey, val); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}

// scanTitleSourceRequest /settings/scan-title-source PUT 请求体
type scanTitleSourceRequest struct {
	TitleSource string `json:"title_source" example:"tag" enums:"tag,filename"`
}

// GetScanTitleSourceSetting GET /api/v1/settings/scan-title-source
// @Summary 获取扫描标题来源配置
// @Description tag：优先使用音频标签中的标题（默认）；filename：始终使用文件名（不含扩展名）作为标题。切换后需以「重新导入」模式扫描才能生效。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} scanTitleSourceRequest "返回 title_source 字段"
// @Security BearerAuth
// @Router /settings/scan-title-source [get]
func (h *ScanHandler) GetScanTitleSourceSetting(w http.ResponseWriter, r *http.Request) {
	titleSource := "tag"
	if h.configService != nil {
		titleSource = h.configService.GetString(scanTitleSourceConfigKey, "tag")
	}
	respondJSON(w, http.StatusOK, scanTitleSourceRequest{TitleSource: titleSource})
}

// UpdateScanTitleSourceSetting PUT /api/v1/settings/scan-title-source
// @Summary 更新扫描标题来源配置
// @Description tag：优先使用音频标签中的标题；filename：始终使用文件名（不含扩展名）作为标题。切换后需以「重新导入」模式扫描才能生效。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body scanTitleSourceRequest true "标题来源配置"
// @Success 200 {object} scanTitleSourceRequest "返回 title_source 字段"
// @Failure 400 {object} map[string]string "请求格式错误或参数无效"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/scan-title-source [put]
func (h *ScanHandler) UpdateScanTitleSourceSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req scanTitleSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.TitleSource != "tag" && req.TitleSource != "filename" {
		respondError(w, http.StatusBadRequest, "title_source 必须为 tag 或 filename", nil)
		return
	}
	if err := h.configService.Set(scanTitleSourceConfigKey, req.TitleSource); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	if h.onMusicPathChanged != nil {
		go h.onMusicPathChanged()
	}
	respondJSON(w, http.StatusOK, scanTitleSourceRequest{TitleSource: req.TitleSource})
}

// FingerprintStatus /scan/fingerprints/status 的响应体。
type FingerprintStatus struct {
	// ChromaprintAvailable ffmpeg 是否带 chromaprint muxer，false 时无法计算指纹
	ChromaprintAvailable bool `json:"chromaprint_available"`
	// Total 本地歌曲总数
	Total int64 `json:"total"`
	// Computed 已有指纹的数量
	Computed int64 `json:"computed"`
	// Missing 尚未尝试过计算的数量（= total - computed - failed）
	Missing int64 `json:"missing"`
	// Failed 尝试过但失败的数量（无音轨 / 文件损坏 / 超时），不会自动重试，
	// 需要「重新计算全部」才会再试
	Failed int64 `json:"failed"`
	// AutoEnabled 扫描后是否自动计算指纹（config scan_auto_fingerprint）
	AutoEnabled bool `json:"auto_enabled"`
}

// GetFingerprintStatus 获取指纹计算状态
// @Summary 获取指纹计算状态
// @Description 返回 ffmpeg chromaprint 可用性、本地歌曲指纹计算统计（含尝试失败数）以及「扫描后自动计算指纹」开关状态
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} FingerprintStatus "指纹状态"
// @Failure 500 {object} map[string]string "查询指纹统计失败"
// @Security BearerAuth
// @Router /scan/fingerprints/status [get]
func (h *ScanHandler) GetFingerprintStatus(w http.ResponseWriter, r *http.Request) {
	status := FingerprintStatus{ChromaprintAvailable: services.IsChromaprintAvailable()}
	if h.songService != nil {
		total, computed, failed, err := h.songService.CountLocalFingerprints(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, "查询指纹统计失败", err)
			return
		}
		status.Total = total
		status.Computed = computed
		status.Failed = failed
		status.Missing = total - computed - failed
	}
	if h.configService != nil {
		status.AutoEnabled = h.configService.GetBool(scanAutoFingerprintConfigKey, false)
	}
	respondJSON(w, http.StatusOK, status)
}

// CancelFingerprintCompute 中断正在运行的指纹计算
// @Summary 中断指纹计算
// @Description 停止正在运行的批量指纹计算任务并杀掉其 ffmpeg 子进程。指纹任务不挂在扫描的取消通道上（扫描「完成」后该通道已关闭），所以需要独立的取消入口。任务不在运行时返回 cancelled=false。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} map[string]bool "返回 cancelled 字段"
// @Security BearerAuth
// @Router /scan/fingerprints/cancel [post]
func (h *ScanHandler) CancelFingerprintCompute(w http.ResponseWriter, r *http.Request) {
	if h.fingerprintService == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"cancelled": false})
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"cancelled": h.fingerprintService.Cancel()})
}

// StartFingerprintCompute 触发批量指纹计算
// @Summary 触发批量指纹计算
// @Description 异步为本地歌曲计算音频指纹，需要 ffmpeg 支持 chromaprint。若已有任务在运行则打断重启。传入 recompute_all=true 时清空已有指纹后重新计算全部；传入 retry_failed=true 时仅重置失败项的「已尝试」标记后重试（已算好的指纹保留，适用于 ffmpeg 能力升级后恢复失败歌曲）。两者同时传入时 recompute_all 优先。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body handlers.startFingerprintRequest false "计算选项"
// @Success 200 {object} map[string]interface{} "任务已启动"
// @Failure 400 {object} map[string]string "chromaprint 不可用"
// @Security BearerAuth
// @Router /scan/fingerprints [post]
func (h *ScanHandler) StartFingerprintCompute(w http.ResponseWriter, r *http.Request) {
	if !services.IsChromaprintAvailable() {
		respondError(w, http.StatusBadRequest, "ffmpeg chromaprint 不可用，无法计算音频指纹", nil)
		return
	}
	if h.fingerprintService == nil {
		respondError(w, http.StatusInternalServerError, "fingerprint service not initialized", nil)
		return
	}

	var req startFingerprintRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "请求体格式错误", err)
			return
		}
	}

	var total int
	var err error
	switch {
	case req.RecomputeAll:
		total, err = h.fingerprintService.RecomputeAll()
	case req.RetryFailed:
		total, err = h.fingerprintService.RetryFailed()
	default:
		total, err = h.fingerprintService.ComputeMissing()
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "started",
		"total":  total,
	})
}

type startFingerprintRequest struct {
	RecomputeAll bool `json:"recompute_all"`
	// RetryFailed 仅重置失败项的「已尝试」标记后重试，已算好的指纹保留。
	RetryFailed bool `json:"retry_failed"`
}

// GetFingerprintProgress 获取指纹计算进度
// @Summary 获取指纹计算进度
// @Description 查询当前指纹计算任务的进度
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} services.FingerprintProgress "计算进度"
// @Security BearerAuth
// @Router /scan/fingerprints/progress [get]
func (h *ScanHandler) GetFingerprintProgress(w http.ResponseWriter, r *http.Request) {
	if h.fingerprintService == nil {
		respondJSON(w, http.StatusOK, services.FingerprintProgress{Status: "idle"})
		return
	}
	respondJSON(w, http.StatusOK, h.fingerprintService.GetProgress())
}

// FailedFingerprintItem 指纹计算失败的歌曲信息（API 响应用）。
type FailedFingerprintItem struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	FilePath    string `json:"file_path"`
	Error       string `json:"error"`
	AttemptedAt int64  `json:"attempted_at"`
}

// GetFailedFingerprints 获取指纹计算失败的歌曲列表
// @Summary 获取指纹计算失败的歌曲列表
// @Description 返回所有指纹计算失败的本地歌曲，包含失败原因。用于在重复检测页面展示失败详情。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} map[string]any "成功返回 {items: [...], total: N}"
// @Failure 500 {object} map[string]string "服务器错误"
// @Security BearerAuth
// @Router /scan/fingerprints/failed [get]
func (h *ScanHandler) GetFailedFingerprints(w http.ResponseWriter, r *http.Request) {
	rows, err := h.songService.ListFailedFingerprints(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "获取失败列表失败", err)
		return
	}
	items := make([]FailedFingerprintItem, len(rows))
	for i, row := range rows {
		items[i] = FailedFingerprintItem{
			ID:          row.ID,
			Title:       row.Title,
			Artist:      row.Artist,
			FilePath:    row.FilePath,
			Error:       row.FingerprintError,
			AttemptedAt: row.FingerprintAttemptedAt,
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

// AutoScanSetting /settings/auto-scan 的请求与响应体。
type AutoScanSetting struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
}

// GetAutoScanSetting GET /api/v1/settings/auto-scan
// @Summary 获取自动扫描配置
// @Description 返回自动扫描的启用状态和扫描间隔（秒）。默认关闭，间隔 3600 秒（1 小时）。
// @Tags 扫描管理
// @Produce json
// @Success 200 {object} AutoScanSetting
// @Security BearerAuth
// @Router /settings/auto-scan [get]
func (h *ScanHandler) GetAutoScanSetting(w http.ResponseWriter, r *http.Request) {
	cfg := AutoScanSetting{
		Enabled:         false,
		IntervalSeconds: 3600,
	}
	if h.configService != nil {
		_ = h.configService.GetJSON(autoScanConfigKey, &cfg)
	}
	respondJSON(w, http.StatusOK, cfg)
}

// UpdateAutoScanSetting PUT /api/v1/settings/auto-scan
// @Summary 更新自动扫描配置
// @Description 设置自动扫描的启用状态和扫描间隔。interval_seconds 有效范围 [60, 86400]。更新后立即生效（无需重启）。
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body AutoScanSetting true "自动扫描配置"
// @Success 200 {object} AutoScanSetting
// @Failure 400 {object} map[string]string "请求格式错误或参数无效"
// @Failure 500 {object} map[string]string "保存配置失败"
// @Security BearerAuth
// @Router /settings/auto-scan [put]
func (h *ScanHandler) UpdateAutoScanSetting(w http.ResponseWriter, r *http.Request) {
	if h.configService == nil {
		respondError(w, http.StatusInternalServerError, "configService 未注入", nil)
		return
	}
	var req AutoScanSetting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "请求格式错误", err)
		return
	}
	if req.IntervalSeconds < 60 || req.IntervalSeconds > 86400 {
		respondError(w, http.StatusBadRequest, "interval_seconds 必须在 60 到 86400 之间", nil)
		return
	}
	if err := h.configService.SetJSON(autoScanConfigKey, req); err != nil {
		respondError(w, http.StatusInternalServerError, "保存配置失败", err)
		return
	}
	if h.onAutoScanChanged != nil {
		cfg := services.AutoScanConfig{
			Enabled:         req.Enabled,
			IntervalSeconds: req.IntervalSeconds,
		}
		go h.onAutoScanChanged(cfg)
	}
	respondJSON(w, http.StatusOK, req)
}
