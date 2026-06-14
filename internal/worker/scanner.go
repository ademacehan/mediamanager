package worker

import (
	"MediaManager/internal/repository"
	. "MediaManager/internal/service"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Scan(repo *repository.MediaRepository, rootPath string) error {

	return filepath.Walk(rootPath,
		func(path string, info os.FileInfo, err error) error {

			if err != nil {
				fmt.Printf("Erişim/Yetki Hatası: %s -> %v\n", path, err)
				return nil // Hatayı logla ve devam et
			}

			// sadece dosyalar
			if info.IsDir() {
				return nil
			}

			// Uzantıyı küçük harfe çevirerek al (Örn: .JPG -> .jpg)
			_ = strings.ToLower(filepath.Ext(path)) 
			fileName := info.Name()
			fileSize := info.Size()
			modTime := info.ModTime()
			Hash, err := GenerateFileHash(path)
			if err != nil {
				fmt.Printf("Dosya Okuma/Hash Hatası: %s -> %v\n", path, err)
				return nil // Sıradaki dosyaya geç
			}

			// Repository üzerinden Hash kaydı
			mediaHashID, err := repo.UpsertMediaHash(Hash)
			if err != nil {
				fmt.Printf("DB Hash Kayıt Hatası: %s -> %v\n", path, err)
				return nil // İşlemi durdurma, devam et
			}

			// Repository üzerinden Scan kaydı
			err = repo.UpsertMediaScan(mediaHashID, repository.MediaScanRecord{
				FilePath:         path,
				FileName:         fileName,
				FileSize:         fileSize,
				FileLastModified: modTime,
				FileFirstCreate:  modTime,
			})

			if err != nil {
				fmt.Printf("Scan Kayıt Hatası (%s): %v\n", path, err)
			}

			return nil
		},
	)
}
