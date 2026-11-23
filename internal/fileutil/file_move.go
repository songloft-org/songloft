package fileutil

import (
	"io"
	"os"
)

// MoveFile 尝试 os.Rename，若因跨设备失败则回退到 copy + remove。
func MoveFile(src, dst string) error {
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

// LinkOrCopyExclusive installs a copy at an unused path. Linking avoids copying
// the payload when the source and target share a filesystem with hard links.
func LinkOrCopyExclusive(src, dst string) error {
	if err := os.Link(src, dst); err == nil || os.IsExist(err) {
		return err
	}
	return CopyFileExclusive(src, dst)
}

// CopyFileExclusive creates an independent copy without replacing the destination.
func CopyFileExclusive(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dst)
	}
	return err
}
