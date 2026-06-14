package repository

import (
	"fmt"
	"os"
)

type FileRepository struct{}

func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

func (r *FileRepository) Remove(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("dosya silinemedi: %v", err)
	}
	return nil
}

func (r *FileRepository) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (r *FileRepository) EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
