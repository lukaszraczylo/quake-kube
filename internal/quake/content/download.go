package content

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	httputil "github.com/lukaszraczylo/quake-kube/internal/util/net/http"
)

func CopyAssets(u *url.URL, dir string) error {
	url := strings.TrimSuffix(u.String(), "/")
	files, err := getManifest(url)
	if err != nil {
		return err
	}

	for _, f := range files {
		path, err := safeJoin(dir, f.Name)
		if err != nil {
			return fmt.Errorf("manifest entry %q: %w", f.Name, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			continue
		}
		data, err := fetchAsset(url, f)
		if err != nil {
			return err
		}
		if sum := crc32.ChecksumIEEE(data); sum != f.Checksum {
			return fmt.Errorf("checksum mismatch for %s: got %d, want %d", f.Name, sum, f.Checksum)
		}
		if err := writeFileAtomic(path, bytes.NewReader(data)); err != nil {
			return err
		}
		if strings.HasPrefix(f.Name, "linuxq3ademo") {
			continue
			// if err := extractDemoPack(path, dir); err != nil {
			// 	return err
			// }
		}
		if strings.HasPrefix(f.Name, "linuxq3apoint") {
			if err := extractPointPacks(path, dir); err != nil {
				return err
			}
		}
	}
	return nil
}

// fetchAsset tries the checksum-prefixed-basename route this content server
// serves (baseq3/<crc>-pak0.pk3), then the legacy prefixed-path route.
func fetchAsset(base string, f *File) ([]byte, error) {
	dir, name := path.Split(f.Name)
	data, err := httputil.GetBody(base + fmt.Sprintf("/assets/%s%d-%s", dir, f.Checksum, name))
	if err == nil {
		return data, nil
	}
	legacy, lerr := httputil.GetBody(base + fmt.Sprintf("/assets/%d-%s", f.Checksum, f.Name))
	if lerr != nil {
		return nil, err
	}
	return legacy, nil
}

func getManifest(url string) ([]*File, error) {
	data, err := httputil.GetBody(url + "/assets/manifest.json")
	if err != nil {
		return nil, err
	}

	files := make([]*File, 0)
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s/assets/manifest.json: %w", url, err)
	}
	return files, nil
}

var gzipMagicHeader = []byte{'\x1f', '\x8b'}

func extractPointPacks(path, dir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	idx := bytes.Index(data, gzipMagicHeader)
	if idx < 0 {
		return fmt.Errorf("no gzip payload in %s", path)
	}
	data = data[idx:]
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()

	data, err = io.ReadAll(gr)
	if err != nil {
		return err
	}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.HasSuffix(hdr.Name, ".pk3") {
			fmt.Printf("Downloaded %s\n", hdr.Name)
			data, err := io.ReadAll(tr)
			if err != nil {
				return err
			}
			path, err := safeJoin(dir, hdr.Name)
			if err != nil {
				return err
			}
			if err := writeFileAtomic(path, bytes.NewReader(data)); err != nil {
				return err
			}
		}
	}
	return nil
}
