package content

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type File struct {
	Name       string `json:"name"`
	Compressed int64  `json:"compressed"`
	Checksum   uint32 `json:"checksum"`
}

// store caches the assets manifest and map list for one directory. The cache
// is rebuilt on first use and after invalidate, so uploads become visible.
type store struct {
	dir        string
	assets     []*File
	maps       []*Map
	assetsJSON []byte

	mu    sync.Mutex
	valid bool
}

func newStore(dir string) *store { return &store{dir: dir} }

func (s *store) invalidate() {
	s.mu.Lock()
	s.valid = false
	s.mu.Unlock()
}

// manifest returns the JSON-encoded assets manifest.
func (s *store) manifest() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return nil, err
	}
	return s.assetsJSON, nil
}

// mapList returns every map found in the map packs.
func (s *store) mapList() ([]*Map, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return nil, err
	}
	return s.maps, nil
}

func (s *store) refresh() error {
	if s.valid {
		return nil
	}
	assets, err := getAssets(s.dir)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(assets, "", "   ")
	if err != nil {
		return err
	}
	maps, err := getMaps(s.dir)
	if err != nil {
		return err
	}
	s.assets, s.assetsJSON, s.maps, s.valid = assets, data, maps, true
	return nil
}

func getAssets(dir string) ([]*File, error) {
	files := []*File{}
	err := walk(dir, func(path string, info os.FileInfo, _ error) error {
		sum, err := checksum(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, &File{filepath.ToSlash(rel), info.Size(), sum})
		return nil
	}, ".pk3", ".sh", ".run")
	return files, err
}

func checksum(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer closeQuietly(f)
	h := crc32.NewIEEE()
	if _, err := io.Copy(h, f); err != nil {
		return 0, fmt.Errorf("checksum %s: %w", path, err)
	}
	return h.Sum32(), nil
}

func getMaps(dir string) ([]*Map, error) {
	result := []*Map{}
	err := walk(dir, func(path string, _ os.FileInfo, _ error) error {
		maps, err := packMaps(path)
		if err != nil {
			// One corrupt pack must not hide every other map.
			log.Printf("content: skipping %s: %v", path, err)
			return nil
		}
		result = append(result, maps...)
		return nil
	}, ".pk3")
	return result, err
}

func packMaps(path string) ([]*Map, error) {
	mp, err := OpenMapPack(path)
	if err != nil {
		return nil, err
	}
	defer closeQuietly(mp)
	return mp.Maps()
}

// closeQuietly closes a handle whose close error is not actionable.
func closeQuietly(c io.Closer) { _ = c.Close() }

func hasExts(path string, exts ...string) bool {
	path = strings.ToLower(path)
	for _, ext := range exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func walk(root string, walkFn filepath.WalkFunc, exts ...string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		if info.IsDir() || !hasExts(path, exts...) {
			return nil
		}
		return walkFn(path, info, err)
	})
}
