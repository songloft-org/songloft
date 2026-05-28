package services

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// moveFile 尝试 os.Rename，若因跨设备失败则回退到 copy + remove。
func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !isCrossDeviceError(err) {
		return err
	}
	if err := copyFileRaw(src, dst); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

func isCrossDeviceError(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

func copyFileRaw(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(df, sf); err != nil {
		df.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := df.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}
