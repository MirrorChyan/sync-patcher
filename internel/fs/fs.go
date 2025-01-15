package fs

import (
	. "mirrorc-sync/internel/shared"
	"os"
	"path"
	"path/filepath"
	"sort"
)

type pl []PendingFileInfo

var _ sort.Interface = (*pl)(nil)

func (p pl) Len() int {
	return len(p)
}

func (p pl) Less(i, j int) bool {
	return p[i].Attr > p[j].Attr
}

func (p pl) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func CollectFileList(dir string) (map[string]int32, error) {
	var files = make(map[string]int32)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." || rel == ".." {
			return nil
		}

		if !info.IsDir() {
			files[rel] = TFile
		} else {
			d, err := os.ReadDir(p)
			if err != nil {
				return err
			}
			if len(d) == 0 {
				files[rel] = TEmptyDir
			} else {
				files[rel] = TDir
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func GetPendingFileList(source, dest map[string]int32) []PendingFileInfo {
	var list []PendingFileInfo
	for p, attr := range source {
		_, exist := dest[p]
		info := PendingFileInfo{
			Path:  p,
			Attr:  attr,
			Exist: exist,
		}

		list = append(list, info)

	}
	sort.Sort(pl(list))
	return list
}

func GetBeRemovedFileInfo(source, dest map[string]int32) []DiffFileInfo {
	var (
		removed []DiffFileInfo
	)
	for k, d := range dest {
		if s, ok := source[k]; !ok || d != s {
			removed = append(removed, DiffFileInfo{
				Path: k,
			})
		}
	}

	return removed
}

func ClearDestDir(base string, list []DiffFileInfo) error {
	for _, f := range list {
		err := os.RemoveAll(path.Join(base, f.Path))
		if err != nil {
			return err
		}
	}
	return nil
}
