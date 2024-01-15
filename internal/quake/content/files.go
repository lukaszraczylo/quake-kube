package content

import (
	"hash/crc32"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	Name       string `json:"name"`
	Compressed int64  `json:"compressed"`
	Checksum   uint32 `json:"checksum"`
}

var assetsList = []*File{}

func getAssets(dir string) (files []*File, err error) {
	if len(assetsList) == 0 {
		err = walk(dir, func(path string, info os.FileInfo, err error) error {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			n := crc32.ChecksumIEEE(data)
			path = strings.TrimPrefix(path, dir+"/")
			files = append(files, &File{path, info.Size(), n})
			assetsList = files
			return nil
		}, ".pk3", ".sh", ".run")
	} else {
		files = assetsList
	}
	return
}

func hasExts(path string, exts ...string) bool {
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
		if !hasExts(path, exts...) {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		return walkFn(path, info, err)
	})
}
