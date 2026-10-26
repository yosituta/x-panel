package service

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
)

// BackupService 数据快照与还原：用 SQLite 的 VACUUM INTO 生成一致性快照，
// 再 gzip 压缩保存，不依赖外部 sqlite3 命令。
type BackupService struct{}

const (
	backupDirName    = "backups"
	// DefaultBackupKeep 默认保留的快照份数
	DefaultBackupKeep = 10
	sqliteHeader     = "SQLite format 3"
)

// Snapshot 一份快照的元信息
type Snapshot struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time int64  `json:"time"`
}

func (s *BackupService) dir() string {
	return filepath.Join(config.GetDBFolderPath(), backupDirName)
}

// Create 生成一份快照，并按 keep 份数清理旧文件
func (s *BackupService) Create(keep int) (*Snapshot, error) {
	if keep <= 0 {
		keep = DefaultBackupKeep
	}
	dir := s.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	tmp := filepath.Join(dir, fmt.Sprintf(".vacuum-%d.db", time.Now().UnixNano()))
	defer os.Remove(tmp)
	// VACUUM INTO 生成的是完整且一致的数据库副本，面板运行中也安全
	if err := database.GetDB().Exec("VACUUM INTO ?", tmp).Error; err != nil {
		return nil, fmt.Errorf("生成快照失败: %w", err)
	}

	name := fmt.Sprintf("x-ui-%s.db.gz", time.Now().Format("20060102-150405"))
	final := filepath.Join(dir, name)
	if err := gzipFile(tmp, final); err != nil {
		return nil, err
	}
	if err := s.prune(keep); err != nil {
		logger.Warning("清理旧快照失败:", err)
	}
	st, err := os.Stat(final)
	if err != nil {
		return nil, err
	}
	return &Snapshot{Name: name, Size: st.Size(), Time: st.ModTime().UnixMilli()}, nil
}

// List 列出全部快照，按时间倒序
func (s *BackupService) List() ([]Snapshot, error) {
	entries, err := os.ReadDir(s.dir())
	if err != nil {
		if os.IsNotExist(err) {
			return []Snapshot{}, nil
		}
		return nil, err
	}
	list := make([]Snapshot, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		list = append(list, Snapshot{Name: e.Name(), Size: info.Size(), Time: info.ModTime().UnixMilli()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Time > list[j].Time })
	return list, nil
}

// Restore 用指定快照覆盖当前数据库；调用方负责重启面板
func (s *BackupService) Restore(name string) error {
	// 只允许还原本目录下的快照文件名，避免路径穿越
	if name != filepath.Base(name) || !strings.HasSuffix(name, ".db.gz") {
		return fmt.Errorf("非法的快照名称: %s", name)
	}
	src := filepath.Join(s.dir(), name)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("快照不存在: %s", name)
	}

	tmp := filepath.Join(s.dir(), fmt.Sprintf(".restore-%d.db", time.Now().UnixNano()))
	defer os.Remove(tmp)
	if err := gunzipFile(src, tmp); err != nil {
		return err
	}
	if err := checkSQLite(tmp); err != nil {
		return err
	}

	dbPath := config.GetDBPath()
	// 先把当前库另存一份，误操作还能救回来
	aside := fmt.Sprintf("%s.before-restore-%s", dbPath, time.Now().Format("20060102-150405"))
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Rename(dbPath, aside); err != nil {
			return fmt.Errorf("备份当前数据库失败: %w", err)
		}
	}
	if err := copyFile(tmp, dbPath); err != nil {
		// 失败时把原库放回去
		_ = os.Rename(aside, dbPath)
		return fmt.Errorf("覆盖数据库失败: %w", err)
	}
	// SQLite 的 -wal / -shm 必须一起清掉，否则会和旧数据混在一起
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	logger.Info("数据库已从快照还原:", name, "（原库保留在", aside, "）")
	return nil
}

// prune 只保留最近 keep 份
func (s *BackupService) prune(keep int) error {
	list, err := s.List()
	if err != nil {
		return err
	}
	for i := keep; i < len(list); i++ {
		if err := os.Remove(filepath.Join(s.dir(), list[i].Name)); err != nil {
			return err
		}
	}
	return nil
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := gzip.NewWriter(out)
	if _, err := io.Copy(zw, in); err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

func gunzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	zr, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("快照解压失败: %w", err)
	}
	defer zr.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, zr); err != nil {
		return err
	}
	return nil
}

func checkSQLite(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, len(sqliteHeader))
	if _, err := io.ReadFull(f, buf); err != nil {
		return fmt.Errorf("快照文件不完整")
	}
	if string(buf) != sqliteHeader {
		return fmt.Errorf("快照不是有效的 SQLite 数据库")
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
