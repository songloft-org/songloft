package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withFixedNow 临时替换 nowFunc，返回恢复函数。
func withFixedNow(t *testing.T, ts time.Time) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return ts }
	t.Cleanup(func() { nowFunc = prev })
}

func TestRotateWriter_WriteAndList(t *testing.T) {
	dir := t.TempDir()
	withFixedNow(t, time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC))

	w, err := NewRotateWriter(dir, 0, 0)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := w.Write([]byte("world\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatalf("ListLogFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("期望 1 个日志文件，得到 %d: %v", len(files), files)
	}
	if !strings.HasSuffix(files[0], "songloft-2026-07-22.log") {
		t.Errorf("文件名不符: %s", files[0])
	}
	content, _ := os.ReadFile(files[0])
	if string(content) != "hello\nworld\n" {
		t.Errorf("内容不符: %q", content)
	}
}

func TestRotateWriter_SizeRotation(t *testing.T) {
	dir := t.TempDir()
	withFixedNow(t, time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC))

	w, err := NewRotateWriter(dir, 10, 0) // 10 字节即轮转
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	defer w.Close()

	w.Write([]byte("12345678\n")) // 9 字节，主文件
	w.Write([]byte("abcdefgh\n")) // 触发 size 轮转（9+9 > 10），归档旧文件后写新主文件

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatalf("ListLogFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("期望 2 个文件（主 + 归档），得到 %d: %v", len(files), files)
	}
	// 归档文件内容更旧，按文件名升序排在主文件之前（.1.log < .log）
	if !strings.Contains(files[0], ".1.log") {
		t.Errorf("期望第一个为归档文件（更旧），得到: %v", files)
	}
	if !strings.HasSuffix(files[1], "songloft-2026-07-22.log") {
		t.Errorf("期望第二个为当日主文件，得到: %v", files)
	}
}

func TestRotateWriter_DateRollover(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 7, 22, 23, 59, 0, 0, time.UTC)
	withFixedNow(t, base)

	w, err := NewRotateWriter(dir, 0, 0)
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	defer w.Close()

	w.Write([]byte("day1\n"))
	withFixedNow(t, base.Add(2*time.Minute)) // 跨天
	w.Write([]byte("day2\n"))

	files, err := ListLogFiles(dir)
	if err != nil {
		t.Fatalf("ListLogFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("跨天应产生 2 个文件，得到 %d: %v", len(files), files)
	}
}

func TestRotateWriter_CleanOld(t *testing.T) {
	dir := t.TempDir()
	// 预置一个 10 天前的旧日志文件
	old := filepath.Join(dir, "songloft-2000-01-01.log")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(old, oldTime, oldTime)

	withFixedNow(t, time.Now())
	w, err := NewRotateWriter(dir, 0, 7) // 保留 7 天
	if err != nil {
		t.Fatalf("NewRotateWriter: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("超期日志文件应被清理，但仍存在: %v", err)
	}
}
