// Package logging 提供后端日志落盘与脱敏能力。
//
// 后端 slog 默认只写 os.Stdout（依赖 Docker/systemd 采集）。为支持「导出日志供用户
// 提交 issue」，本包提供一个自研的按天轮转文件 writer（RotateWriter），通过
// io.MultiWriter 与 stdout 并联落盘到 <data_dir>/logs/，并提供脱敏（redact.go）。
//
// 设计对齐前端 Flutter 侧的 FileLogger：按天分文件、单文件大小上限、保留 N 天。
// 不引入第三方轮转库（如 lumberjack），保持项目「最小依赖」的约定。
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// filePrefix 日志文件名前缀，导出端点据此枚举本包产出的日志文件。
	filePrefix = "songloft-"
	// fileExt 日志文件扩展名。
	fileExt = ".log"
	// dateLayout 文件名中的日期部分（当天主文件名 = songloft-YYYY-MM-DD.log）。
	dateLayout = "2006-01-02"

	// DefaultMaxSizeBytes 单个日志文件大小上限，超过后轮转到带序号的历史文件。
	DefaultMaxSizeBytes int64 = 20 << 20 // 20 MiB
	// DefaultRetentionDays 日志保留天数，超期文件在启动与轮转时清理。
	DefaultRetentionDays = 7
)

// RotateWriter 是并发安全的按天 + 按大小轮转的日志文件 writer，实现 io.Writer。
//
// 落盘规则：
//   - 当天主文件名为 songloft-YYYY-MM-DD.log，追加写入。
//   - 跨天（日期变化）时切换到新日期的主文件。
//   - 当天主文件超过 maxSize 时，将其重命名为 songloft-YYYY-MM-DD.<seq>.log 归档，
//     再新建空的主文件继续写。
//   - 每次轮转与初始化时清理修改时间早于 retentionDays 的本包日志文件。
type RotateWriter struct {
	dir           string
	maxSize       int64
	retentionDays int

	mu       sync.Mutex
	file     *os.File
	curDate  string // 当前主文件对应日期（dateLayout）
	curSize  int64
	openErrN int // 连续打开失败计数，避免每行都尝试打开刷屏
}

// NewRotateWriter 创建 RotateWriter。dir 会被自动创建；maxSize<=0 用默认值，
// retentionDays<=0 用默认值。创建失败（如目录不可写）时返回错误，调用方应据此
// 退化为仅 stdout（不阻塞启动）。
func NewRotateWriter(dir string, maxSize int64, retentionDays int) (*RotateWriter, error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxSizeBytes
	}
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	w := &RotateWriter{
		dir:           dir,
		maxSize:       maxSize,
		retentionDays: retentionDays,
	}
	w.cleanOld()
	return w, nil
}

// Dir 返回日志落盘目录。
func (w *RotateWriter) Dir() string { return w.dir }

// Write 实现 io.Writer。写入失败时返回错误交给 slog 内部（slog 会丢弃错误，
// 不会因单次落盘失败中断进程），但仍返回声明长度以避免 MultiWriter 中断 stdout。
func (w *RotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := nowFunc().Format(dateLayout)
	if w.file == nil || w.curDate != today {
		if err := w.openForDate(today); err != nil {
			// 打开失败：不阻塞 stdout 输出，声明已写入全部字节。
			w.openErrN++
			return len(p), nil
		}
	}
	if w.curSize+int64(len(p)) > w.maxSize {
		w.rotateBySize()
		// rotateBySize 内部已重开主文件；若失败则 file 置空，下次 Write 重试。
		if w.file == nil {
			return len(p), nil
		}
	}

	n, err := w.file.Write(p)
	w.curSize += int64(n)
	if err != nil {
		return len(p), nil
	}
	return n, nil
}

// Close 关闭当前文件句柄。
func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// mainFileName 返回给定日期的主文件名。
func (w *RotateWriter) mainFileName(date string) string {
	return filePrefix + date + fileExt
}

// openForDate 打开（或创建）指定日期的主文件，更新 curDate/curSize，并清理超期文件。
func (w *RotateWriter) openForDate(date string) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	path := filepath.Join(w.dir, w.mainFileName(date))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, statErr := f.Stat()
	var size int64
	if statErr == nil {
		size = info.Size()
	}
	w.file = f
	w.curDate = date
	w.curSize = size
	w.openErrN = 0
	w.cleanOld()
	return nil
}

// rotateBySize 将当前主文件归档为带序号的历史文件，然后重开空主文件。
func (w *RotateWriter) rotateBySize() {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	main := filepath.Join(w.dir, w.mainFileName(w.curDate))
	// 找一个未占用的序号：songloft-YYYY-MM-DD.<seq>.log
	seq := 1
	var archived string
	for {
		archived = filepath.Join(w.dir, fmt.Sprintf("%s%s.%d%s", filePrefix, w.curDate, seq, fileExt))
		if _, err := os.Stat(archived); os.IsNotExist(err) {
			break
		}
		seq++
	}
	if err := os.Rename(main, archived); err != nil {
		// 归档失败：尝试直接重开主文件（截断），避免无限增长。
		f, openErr := os.OpenFile(main, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if openErr != nil {
			return
		}
		w.file = f
		w.curSize = 0
		return
	}
	f, err := os.OpenFile(main, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	w.file = f
	w.curSize = 0
	w.cleanOld()
}

// cleanOld 删除修改时间早于 retentionDays 的本包日志文件。调用方需持有 mu。
func (w *RotateWriter) cleanOld() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	cutoff := nowFunc().AddDate(0, 0, -w.retentionDays)
	for _, e := range entries {
		if e.IsDir() || !isLogFile(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, e.Name()))
		}
	}
}

// isLogFile 判断文件名是否为本包产出的日志文件。
func isLogFile(name string) bool {
	return strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileExt)
}

// nowFunc 便于测试注入固定时间；生产恒为 time.Now。
var nowFunc = time.Now

// ListLogFiles 返回目录下本包产出的所有日志文件的绝对路径，按文件名升序。
// 文件名升序恰好对应时间从旧到新：跨日按日期升序；同日内归档文件
// songloft-DATE.<seq>.log（内容更旧）排在主文件 songloft-DATE.log 之前
// （'.' 后数字的 ASCII 小于 'l'）。供导出端点按时间顺序拼接读取。
func ListLogFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join(dir, n)
	}
	return paths, nil
}
