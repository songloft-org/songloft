package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"mimusic/internal/services"
)

// ScanHandler 扫描处理器
type ScanHandler struct {
	songService *services.SongService
	scanner     *services.Scanner
}

// NewScanHandler 创建扫描处理器
func NewScanHandler(songService *services.SongService, scanner *services.Scanner) *ScanHandler {
	return &ScanHandler{
		songService: songService,
		scanner:     scanner,
	}
}

// SetScanner 更新扫描器引用（配置变更时调用）
func (h *ScanHandler) SetScanner(scanner *services.Scanner) {
	h.scanner = scanner
}

// ScanRequest 扫描请求参数
type ScanRequest struct {
	Reimport bool `json:"reimport"`
}

// ScanAndImport 扫描并导入本地音乐（异步）
// @Summary 扫描并导入本地音乐
// @Description 异步扫描音乐目录并导入新发现的音乐文件到数据库，立即返回，可通过进度接口查询状态
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Param request body ScanRequest false "扫描请求参数"
// @Success 200 {object} map[string]interface{} "扫描任务已启动"
// @Failure 409 {object} map[string]string "扫描正在进行中"
// @Failure 500 {object} map[string]string "启动扫描失败"
// @Security BearerAuth
// @Router /scan [post]
func (h *ScanHandler) ScanAndImport(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var req ScanRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "无效的请求参数", err)
			return
		}
	}

	err := h.songService.ScanAndImportAsync(req.Reimport)
	if err != nil {
		respondError(w, http.StatusConflict, "扫描正在进行中", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "扫描任务已启动",
	})
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
	if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, cleanRoot+string(filepath.Separator)) {
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
// @Description 递归收集音乐目录下所有唯一的目录名称，按字母排序返回，用于排除目录名称的自动补全
// @Tags 扫描管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "目录名称列表"
// @Failure 500 {object} map[string]string "收集目录名称失败"
// @Security BearerAuth
// @Router /scan/dir-names [get]
func (h *ScanHandler) ListDirNames(w http.ResponseWriter, r *http.Request) {
	names, err := h.scanner.CollectAllDirNames(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "收集目录名称失败", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"names": names,
	})
}
