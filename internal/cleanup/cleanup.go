// Package cleanup видаляє старі знімки й записи за віком і сумарним розміром.
package cleanup

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nvr/internal/config"
	"nvr/internal/store"
)

type Service struct {
	dataDir string
	store   *store.Store
	maxAge  time.Duration // 0 = без ліміту
	maxSize int64         // байт, 0 = без ліміту
	every   time.Duration
}

func New(cfg *config.Config, st *store.Store) *Service {
	var maxAge time.Duration
	if cfg.Retention.MaxDays > 0 {
		maxAge = time.Duration(cfg.Retention.MaxDays) * 24 * time.Hour
	}
	return &Service{
		dataDir: cfg.Capture.DataDir,
		store:   st,
		maxAge:  maxAge,
		maxSize: int64(cfg.Retention.MaxSizeGB * 1024 * 1024 * 1024),
		every:   cfg.Retention.Interval,
	}
}

// Run запускає прибирання одразу й далі кожні every, доки ctx не скасують.
func (s *Service) Run(ctx context.Context) {
	if s.maxAge == 0 && s.maxSize == 0 {
		log.Println("cleanup: ліміти не задані — прибирання вимкнено")
		return
	}
	log.Printf("cleanup: вік>%s, розмір<%s, кожні %s",
		humanAge(s.maxAge), humanSize(s.maxSize), s.every)
	s.RunOnce()
	t := time.NewTicker(s.every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.RunOnce()
		}
	}
}

type fileInfo struct {
	path    string
	modTime time.Time
	size    int64
}

// RunOnce виконує один прохід прибирання.
func (s *Service) RunOnce() {
	files := s.scan()

	var deletedAge, deletedSize int
	now := time.Now()

	// 1) за віком
	if s.maxAge > 0 {
		cutoff := now.Add(-s.maxAge)
		kept := files[:0]
		for _, f := range files {
			if f.modTime.Before(cutoff) {
				if s.remove(f.path) {
					deletedAge++
				}
			} else {
				kept = append(kept, f)
			}
		}
		files = kept
	}

	// 2) за сумарним розміром — видаляємо найстаріші, доки не влізе в ліміт
	if s.maxSize > 0 {
		var total int64
		for _, f := range files {
			total += f.size
		}
		if total > s.maxSize {
			sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
			for _, f := range files {
				if total <= s.maxSize {
					break
				}
				if s.remove(f.path) {
					total -= f.size
					deletedSize++
				}
			}
		}
	}

	// 3) прибрати порожні теки за датою та осиротілі записи в БД
	s.removeEmptyDirs()
	if s.maxAge > 0 {
		if n, err := s.store.DeleteEventsWithoutFileOlderThan(now.Add(-s.maxAge)); err == nil && n > 0 {
			log.Printf("cleanup: прибрано %d старих подій руху з БД", n)
		}
	}

	if deletedAge+deletedSize > 0 {
		log.Printf("cleanup: видалено %d файлів (за віком %d, за розміром %d)",
			deletedAge+deletedSize, deletedAge, deletedSize)
	}
}

// scan збирає всі медіа-файли (jpg, mp4) у data/snapshots і data/recordings.
func (s *Service) scan() []fileInfo {
	var out []fileInfo
	for _, sub := range []string{"snapshots", "recordings"} {
		root := filepath.Join(s.dataDir, sub)
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".jpg" && ext != ".mp4" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			out = append(out, fileInfo{path: path, modTime: info.ModTime(), size: info.Size()})
			return nil
		})
	}
	return out
}

// remove видаляє файл і відповідний рядок у БД.
func (s *Service) remove(path string) bool {
	if err := os.Remove(path); err != nil {
		log.Printf("cleanup: видалення %s: %v", path, err)
		return false
	}
	if strings.Contains(path, string(os.PathSeparator)+"snapshots"+string(os.PathSeparator)) {
		s.store.DeleteSnapshotByPath(path)
	} else {
		s.store.DeleteEventByPath(path)
	}
	return true
}

// removeEmptyDirs прибирає порожні теки за датою.
func (s *Service) removeEmptyDirs() {
	for _, sub := range []string{"snapshots", "recordings"} {
		root := filepath.Join(s.dataDir, sub)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if sub, _ := os.ReadDir(dir); len(sub) == 0 {
				os.Remove(dir)
			}
		}
	}
}

func humanAge(d time.Duration) string {
	if d == 0 {
		return "∞"
	}
	return d.String()
}

func humanSize(b int64) string {
	if b == 0 {
		return "∞"
	}
	const gb = 1024 * 1024 * 1024
	return fmt.Sprintf("%.2gГБ", float64(b)/gb)
}
