package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type StorageProvider interface {
	Save(path string, data io.Reader) error
	Get(path string) (io.ReadCloser, bool, error)
	Head(path string) (bool, error)
	List(path string) ([]Entry, error)
	Delete(path string) error
	Walk(path string, walkFn func(path string, info os.FileInfo, err error) error) error
}

type LocalStorage struct {
	BasePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{BasePath: basePath}
}

func (s *LocalStorage) Save(path string, data io.Reader) error {
	fullPath := filepath.Join(s.BasePath, path)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	return err
}

func (s *LocalStorage) Get(path string) (io.ReadCloser, bool, error) {
	fullPath := filepath.Join(s.BasePath, path)
	file, err := os.Open(fullPath)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return file, true, nil
}

func (s *LocalStorage) Head(path string) (bool, error) {
	fullPath := filepath.Join(s.BasePath, path)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *LocalStorage) List(path string) ([]Entry, error) {
	fullPath := filepath.Join(s.BasePath, path)
	stat, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return nil, nil // Not found is not an error, just empty list? Or specific error?
	}
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, nil // Or error "not a directory"
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var result []Entry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	return result, nil
}

func (s *LocalStorage) Delete(path string) error {
	fullPath := filepath.Join(s.BasePath, path)
	return os.RemoveAll(fullPath)
}

func (s *LocalStorage) Walk(path string, walkFn func(path string, info os.FileInfo, err error) error) error {
	fullPath := filepath.Join(s.BasePath, path)
	return filepath.Walk(fullPath, func(wPath string, info os.FileInfo, err error) error {
		relPath, relErr := filepath.Rel(s.BasePath, wPath)
		if relErr != nil {
			return walkFn(wPath, info, relErr)
		}
		return walkFn(relPath, info, err)
	})
}
