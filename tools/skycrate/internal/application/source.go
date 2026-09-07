package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type localFile struct {
	path         string
	relativePath string
	size         int64
}

func collectFiles(ctx context.Context, sourcePath string, sourceInfo os.FileInfo) ([]localFile, error) {
	if sourceInfo.Mode().IsRegular() {
		if err := checkReadable(sourcePath); err != nil {
			return nil, err
		}
		return []localFile{{
			path:         sourcePath,
			relativePath: filepath.Base(sourcePath),
			size:         sourceInfo.Size(),
		}}, nil
	}

	files := []localFile{}
	err := filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("walk directory %q: %w", sourcePath, err)
		}
		if path == sourcePath || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("inspect file %q: symbolic links are not supported", path)
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("inspect file %q: not a regular file", path)
		}
		if err := checkReadable(path); err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("resolve path for %q: %w", path, err)
		}
		files = append(files, localFile{
			path:         path,
			relativePath: filepath.ToSlash(relativePath),
			size:         info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func checkReadable(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %q: %w", filePath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file %q: %w", filePath, err)
	}
	return nil
}
