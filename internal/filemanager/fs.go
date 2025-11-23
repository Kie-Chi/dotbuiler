package filemanager

import (
	"dotbuilder/pkg/logger"
	"io/fs"
	"os"
)

// FileSystem 接口抽象了文件操作
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	Symlink(oldname, newname string) error
	Remove(name string) error
	ReadFile(name string) ([]byte, error)
	Lstat(name string) (fs.FileInfo, error)
	Readlink(name string) (string, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (fs.FileInfo, error)
}

// RealFS 真实文件系统
type RealFS struct{}

func (RealFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (RealFS) Symlink(old, new string) error                { return os.Symlink(old, new) }
func (RealFS) Remove(name string) error                     { return os.Remove(name) }
func (RealFS) ReadFile(name string) ([]byte, error)         { return os.ReadFile(name) }
func (RealFS) Lstat(name string) (fs.FileInfo, error)       { return os.Lstat(name) }
func (RealFS) Readlink(name string) (string, error)         { return os.Readlink(name) }
func (RealFS) WriteFile(n string, d []byte, p os.FileMode) error { return os.WriteFile(n, d, p) }
func (RealFS) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }

// DryRunFS 模拟文件系统
type DryRunFS struct{}

func (DryRunFS) MkdirAll(path string, perm os.FileMode) error {
	logger.Debug("[DryRun] MkdirAll %s", path)
	return nil
}
func (DryRunFS) Symlink(old, new string) error {
	logger.Info("[DryRun] Symlink %s -> %s", new, old)
	return nil
}
func (DryRunFS) Remove(name string) error {
	logger.Info("[DryRun] Remove %s", name)
	return nil
}
func (DryRunFS) ReadFile(name string) ([]byte, error) {
	// 在 DryRun 模式下，如果源文件不存在（可能由前置任务生成），不要报错
	if _, err := os.Stat(name); os.IsNotExist(err) {
		logger.Warn("[DryRun] Source file not found (simulating read): %s", name)
		return []byte("dry-run-content"), nil
	}
	return os.ReadFile(name)
}
func (DryRunFS) Lstat(name string) (fs.FileInfo, error) {
	// 尝试真实读取，以便做出正确判断（例如目标已存在），如果不存在则模拟
	info, err := os.Lstat(name)
	if err != nil && os.IsNotExist(err) {
		return nil, err // 真实的不存在
	}
	return info, err
}
func (DryRunFS) Readlink(name string) (string, error) {
	return os.Readlink(name)
}
func (DryRunFS) WriteFile(n string, d []byte, p os.FileMode) error {
	logger.Info("[DryRun] WriteFile %s (%d bytes)", n, len(d))
	return nil
}
func (DryRunFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}