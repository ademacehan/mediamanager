package service

import (
	"MediaManager/internal/repository"
	"path/filepath"
)

type MediaService struct {
	repo     *repository.MediaRepository
	fileRepo *repository.FileRepository
}

func NewMediaService(repo *repository.MediaRepository, fileRepo *repository.FileRepository) *MediaService {
	return &MediaService{
		repo:     repo,
		fileRepo: fileRepo,
	}
}

func (s *MediaService) DeleteMedia(scanID int64) error {
	record, err := s.repo.GetScanByID(scanID)
	if err != nil {
		return err
	}

	if err := s.fileRepo.Remove(record.FilePath); err != nil {
		return err
	}

	return s.repo.DeleteMediaScan(scanID)
}

func (s *MediaService) RenameMedia(scanID int64, newName string) error {
	record, err := s.repo.GetScanByID(scanID)
	if err != nil {
		return err
	}

	oldPath := filepath.Clean(record.FilePath)
	newPath := filepath.Join(filepath.Dir(oldPath), newName)

	if err := s.fileRepo.Rename(oldPath, newPath); err != nil {
		return err
	}

	return s.repo.UpdateMediaPath(scanID, newPath, newName)
}

func (s *MediaService) MoveMedia(scanID int64, targetDir string) error {
	record, err := s.repo.GetScanByID(scanID)
	if err != nil {
		return err
	}

	if err := s.fileRepo.EnsureDir(targetDir); err != nil {
		return err
	}

	oldPath := filepath.Clean(record.FilePath)
	newPath := filepath.Join(targetDir, record.FileName)

	if err := s.fileRepo.Rename(oldPath, newPath); err != nil {
		return err
	}

	return s.repo.UpdateMediaPath(scanID, newPath, record.FileName)
}

func (s *MediaService) DeleteOtherDuplicates(keepID int64, hashID string) error {
	ids, err := s.repo.GetOtherDuplicateIDs(keepID, hashID)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if err := s.DeleteMedia(id); err != nil {
			return err
		}
	}
	return nil
}
