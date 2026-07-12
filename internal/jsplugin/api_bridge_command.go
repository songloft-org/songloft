package jsplugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"songloft/internal/httputil"
)

const (
	maxExecTimeout     = 300 * time.Second
	defaultExecTimeout = 60 * time.Second
	maxOutputSize      = 10 << 20  // 10MB per stdout/stderr
	maxDownloadSize    = 500 << 20 // 500MB
)

var binFilenameRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type managedProcess struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

func (h *BridgeHandler) binDir() string {
	return filepath.Join(h.dataDir, h.service.plugin.EntryPath, "bin")
}

func (h *BridgeHandler) pluginDataDir() string {
	return filepath.Join(h.dataDir, h.service.plugin.EntryPath)
}

// Cleanup 终止所有后台进程。由 JSService.Stop() 调用。
func (h *BridgeHandler) Cleanup() {
	h.processes.Range(func(key, value any) bool {
		mp := value.(*managedProcess)
		mp.cancel()
		_ = mp.cmd.Wait()
		h.processes.Delete(key)
		slog.Info("killed background process on cleanup", "name", key, "plugin", h.service.plugin.EntryPath)
		return true
	})
	h.cleanupUDPSockets()
	h.cleanupTCPSockets()
	h.cleanupInboundWebSockets()
}

func (h *BridgeHandler) handleCommand(action, data string) (string, error) {
	switch action {
	case "command.exec":
		return h.commandExec(data)
	case "command.start":
		return h.commandStart(data)
	case "command.stop":
		return h.commandStop(data)
	case "command.isRunning":
		return h.commandIsRunning(data)
	case "command.download":
		return h.commandDownload(data)
	case "command.deleteBin":
		return h.commandDeleteBin(data)
	case "command.listBin":
		return h.commandListBin()
	case "command.exists":
		return h.commandExists(data)
	default:
		return "", fmt.Errorf("unknown command action: %s", action)
	}
}

// --- 一次性命令执行 ---

func (h *BridgeHandler) commandExec(data string) (string, error) {
	var req struct {
		Program string            `json:"program"`
		Args    []string          `json:"args"`
		Timeout int               `json:"timeout"` // ms
		Stdin   string            `json:"stdin"`
		Env     map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return "", fmt.Errorf("commandExec: parse request: %w", err)
	}

	resolved, err := h.resolveProgram(req.Program)
	if err != nil {
		return "", fmt.Errorf("commandExec: %w", err)
	}

	timeout := defaultExecTimeout
	if req.Timeout > 0 {
		timeout = min(time.Duration(req.Timeout)*time.Millisecond, maxExecTimeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, resolved, req.Args...)
	cmd.Dir = h.pluginDataDir()
	if len(req.Env) > 0 {
		cmd.Env = append(os.Environ(), mapToEnvSlice(req.Env)...)
	}
	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	stdout := &limitedBuffer{max: maxOutputSize}
	stderr := &limitedBuffer{max: maxOutputSize}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(err, fs.ErrNotExist) {
			if hint := checkELFInterpreter(resolved); hint != "" {
				return "", fmt.Errorf("commandExec: run: %w (%s)", err, hint)
			}
			return "", fmt.Errorf("commandExec: run: %w", err)
		} else {
			return "", fmt.Errorf("commandExec: run: %w", err)
		}
	}

	result, marshalErr := json.Marshal(struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}{exitCode, stdout.String(), stderr.String()})
	if marshalErr != nil {
		return "", fmt.Errorf("commandExec: marshal result: %w", marshalErr)
	}
	return string(result), nil
}

// --- 后台进程管理 ---

func (h *BridgeHandler) commandStart(data string) (string, error) {
	var req struct {
		Name    string            `json:"name"`
		Program string            `json:"program"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return "", fmt.Errorf("commandStart: parse request: %w", err)
	}
	if req.Name == "" {
		return "", fmt.Errorf("commandStart: name is required")
	}

	if _, loaded := h.processes.Load(req.Name); loaded {
		return "", fmt.Errorf("commandStart: process %q is already running", req.Name)
	}

	resolved, err := h.resolveProgram(req.Program)
	if err != nil {
		return "", fmt.Errorf("commandStart: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, resolved, req.Args...)
	cmd.Dir = h.pluginDataDir()
	if len(req.Env) > 0 {
		cmd.Env = append(os.Environ(), mapToEnvSlice(req.Env)...)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		if errors.Is(err, fs.ErrNotExist) {
			if hint := checkELFInterpreter(resolved); hint != "" {
				return "", fmt.Errorf("commandStart: start: %w (%s)", err, hint)
			}
		}
		return "", fmt.Errorf("commandStart: start: %w", err)
	}

	mp := &managedProcess{cmd: cmd, cancel: cancel}
	h.processes.Store(req.Name, mp)

	go func() {
		_ = cmd.Wait()
		h.processes.Delete(req.Name)
		slog.Info("background process exited", "name", req.Name, "plugin", h.service.plugin.EntryPath, "pid", cmd.Process.Pid)
	}()

	result, _ := json.Marshal(struct {
		PID int `json:"pid"`
	}{cmd.Process.Pid})
	return string(result), nil
}

func (h *BridgeHandler) commandStop(data string) (string, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return "", fmt.Errorf("commandStop: parse request: %w", err)
	}

	val, ok := h.processes.Load(req.Name)
	if !ok {
		return "", fmt.Errorf("commandStop: process %q not found", req.Name)
	}

	mp := val.(*managedProcess)
	mp.cancel()
	_ = mp.cmd.Wait()
	h.processes.Delete(req.Name)
	return "", nil
}

func (h *BridgeHandler) commandIsRunning(data string) (string, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return "", fmt.Errorf("commandIsRunning: parse request: %w", err)
	}

	_, ok := h.processes.Load(req.Name)
	result, _ := json.Marshal(ok)
	return string(result), nil
}

// --- 二进制文件管理 ---

func (h *BridgeHandler) commandDownload(data string) (string, error) {
	var req struct {
		URL           string `json:"url"`
		Filename      string `json:"filename"`
		Extract       string `json:"extract"`       // "tgz" | ""
		ExtractTarget string `json:"extractTarget"` // optional: only keep this file
	}
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return "", fmt.Errorf("commandDownload: parse request: %w", err)
	}
	if err := validateBinFilename(req.Filename); err != nil {
		return "", fmt.Errorf("commandDownload: %w", err)
	}
	if req.ExtractTarget != "" {
		if err := validateBinFilename(req.ExtractTarget); err != nil {
			return "", fmt.Errorf("commandDownload: invalid extractTarget: %w", err)
		}
	}

	client := httputil.NewClient(120 * time.Second)
	resp, err := client.Get(req.URL)
	if err != nil {
		return "", fmt.Errorf("commandDownload: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("commandDownload: HTTP %d", resp.StatusCode)
	}

	binDir := h.binDir()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("commandDownload: mkdir: %w", err)
	}

	destPath := filepath.Join(binDir, req.Filename)
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("commandDownload: create file: %w", err)
	}

	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		f.Close()
		os.Remove(destPath)
		return "", fmt.Errorf("commandDownload: write: %w", err)
	}
	_ = f.Sync()
	f.Close()
	if n > maxDownloadSize {
		os.Remove(destPath)
		return "", fmt.Errorf("commandDownload: file exceeds %dMB limit", maxDownloadSize>>20)
	}

	if req.Extract == "tgz" {
		if err := h.extractTgz(destPath, binDir, req.ExtractTarget); err != nil {
			os.Remove(destPath)
			return "", fmt.Errorf("commandDownload: extract tgz: %w", err)
		}
		os.Remove(destPath)
	}

	return "", nil
}

func (h *BridgeHandler) extractTgz(tgzPath, destDir, targetName string) error {
	f, err := os.Open(tgzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	extracted := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		baseName := filepath.Base(hdr.Name)
		if targetName != "" && baseName != targetName {
			continue
		}

		outPath := filepath.Join(destDir, baseName)
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return fmt.Errorf("create %s: %w", baseName, err)
		}
		if _, err := io.Copy(out, io.LimitReader(tr, maxDownloadSize)); err != nil {
			out.Close()
			os.Remove(outPath)
			return fmt.Errorf("write %s: %w", baseName, err)
		}
		_ = out.Sync()
		out.Close()
		extracted++

		if targetName != "" {
			break
		}
	}

	if targetName != "" && extracted == 0 {
		return fmt.Errorf("target %q not found in archive", targetName)
	}
	return nil
}

func (h *BridgeHandler) commandDeleteBin(data string) (string, error) {
	filename := data
	if err := validateBinFilename(filename); err != nil {
		return "", fmt.Errorf("commandDeleteBin: %w", err)
	}
	path := filepath.Join(h.binDir(), filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("commandDeleteBin: %w", err)
	}
	return "", nil
}

func (h *BridgeHandler) commandListBin() (string, error) {
	entries, err := os.ReadDir(h.binDir())
	if err != nil {
		if os.IsNotExist(err) {
			return "[]", nil
		}
		return "", fmt.Errorf("commandListBin: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	result, _ := json.Marshal(names)
	return string(result), nil
}

func (h *BridgeHandler) commandExists(data string) (string, error) {
	filename := data
	if err := validateBinFilename(filename); err != nil {
		return "", fmt.Errorf("commandExists: %w", err)
	}
	_, err := os.Stat(filepath.Join(h.binDir(), filename))
	result, _ := json.Marshal(err == nil)
	return string(result), nil
}

// --- 辅助函数 ---

func (h *BridgeHandler) resolveProgram(program string) (string, error) {
	if program == "" {
		return "", fmt.Errorf("invalid program name: program cannot be empty")
	}

	if filepath.IsAbs(program) {
		return resolveAbsoluteProgram(program)
	}

	if err := validateBinFilename(program); err != nil {
		return "", fmt.Errorf("invalid program name: %w", err)
	}

	binPath := filepath.Join(h.binDir(), program)
	if info, err := os.Stat(binPath); err == nil && !info.IsDir() {
		return binPath, nil
	}

	// Avoid exec.LookPath which uses faccessat2 syscall — blocked by seccomp on Termux.
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, program)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("program %q not found in plugin bin/ or system PATH", program)
}

func resolveAbsoluteProgram(program string) (string, error) {
	if strings.Contains(program, "..") {
		return "", fmt.Errorf("invalid program path: %q contains '..'", program)
	}
	if filepath.Clean(program) != program {
		return "", fmt.Errorf("invalid program path: %q is not clean (expected %q)", program, filepath.Clean(program))
	}
	info, err := os.Stat(program)
	if err != nil {
		return "", fmt.Errorf("program %q not found: %w", program, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("program %q is a directory, not a file", program)
	}
	return program, nil
}

func validateBinFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if !binFilenameRegexp.MatchString(name) {
		return fmt.Errorf("filename %q contains invalid characters (allowed: a-zA-Z0-9._-)", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("filename %q contains '..'", name)
	}
	return nil
}

func mapToEnvSlice(m map[string]string) []string {
	env := make([]string, 0, len(m))
	for k, v := range m {
		env = append(env, k+"="+v)
	}
	return env
}

// limitedBuffer 是一个有大小上限的 bytes.Buffer。
// 超过上限后静默丢弃多余数据，防止 OOM。
type limitedBuffer struct {
	buf bytes.Buffer
	max int
	mu  sync.Mutex
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.max - b.buf.Len()
	if remaining <= 0 {
		return len(p), nil // 静默丢弃
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	b.buf.Write(p)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// checkELFInterpreter 检查 ELF 二进制的动态链接器是否存在。
// execve 返回 ENOENT 但文件自身存在时，通常是 PT_INTERP 指定的解释器缺失
// （如 glibc 编译的二进制在 Alpine/musl 上运行）。
func checkELFInterpreter(path string) string {
	f, err := elf.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	for _, prog := range f.Progs {
		if prog.Type != elf.PT_INTERP {
			continue
		}
		interp := make([]byte, prog.Filesz)
		if _, err := prog.ReadAt(interp, 0); err != nil {
			return ""
		}
		interpPath := strings.TrimRight(string(interp), "\x00")
		if _, err := os.Stat(interpPath); err != nil {
			return fmt.Sprintf("ELF interpreter %q not found — the binary may require glibc but this system uses musl; consider using a statically linked build", interpPath)
		}
		return ""
	}
	return ""
}
