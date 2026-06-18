package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"MediaManager/internal/repository"
	"MediaManager/internal/service"
	"MediaManager/internal/worker"

	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

type ScanRequest struct {
	RootPath string `json:"root_path"`
}

type Config struct {
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
	IndexSuggestion struct {
		ImagePath string `yaml:"image_path"`
		VideoPath string `yaml:"video_path"`
	} `yaml:"index_suggestion"`
	Hashtag struct {
		DefaultHashtag string `yaml:"defhtag"`
	} `yaml:"hashtag"`
}

func isImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

func isVideo(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp4", ".mov", ".avi", ".mkv":
		return true
	}
	return false
}

func main() {
	// Yapılandırmayı oku
	configFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Yapılandırma dosyası okunamadı: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		log.Fatalf("Yapılandırma ayrıştırılamadı: %v", err)
	}

	// Bağlantı dizesini oluştur
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Repository'yi başlat
	mediaRepo := repository.NewMediaRepository(db)
	fileRepo := repository.NewFileRepository()
	mediaService := service.NewMediaService(mediaRepo, fileRepo)

	http.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ScanRequest

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Println("scan started:", req.RootPath)

		go func() {
			err := worker.Scan(mediaRepo, req.RootPath)
			if err != nil {
				log.Println(err)
			}

		}()

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status": "scan started",
		})
	})

	http.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Geçersiz dosya ID'si", http.StatusBadRequest)
			return
		}
		err = mediaService.DeleteMedia(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/delete-others", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		keepIDStr := r.URL.Query().Get("keep_id")
		hashID := r.URL.Query().Get("hash_id")
		keepID, err := strconv.ParseInt(keepIDStr, 10, 64)
		if err != nil || hashID == "" {
			http.Error(w, "Geçersiz parametreler", http.StatusBadRequest)
			return
		}
		err = mediaService.DeleteOtherDuplicates(keepID, hashID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/rename", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := r.URL.Query().Get("id")
		newName := r.URL.Query().Get("new_name")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || newName == "" {
			http.Error(w, "Geçersiz parametreler", http.StatusBadRequest)
			return
		}
		err = mediaService.RenameMedia(id, newName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := r.URL.Query().Get("id")
		targetDir := r.URL.Query().Get("target_dir")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || targetDir == "" {
			http.Error(w, "Geçersiz parametreler", http.StatusBadRequest)
			return
		}
		err = mediaService.MoveMedia(id, targetDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/move-and-delete-others", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := r.URL.Query().Get("id")
		hashID := r.URL.Query().Get("hash_id")
		targetDir := r.URL.Query().Get("target_dir")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || hashID == "" || targetDir == "" {
			http.Error(w, "Geçersiz parametreler", http.StatusBadRequest)
			return
		}
		// 1. Önce seçili dosyayı taşı
		if err := mediaService.MoveMedia(id, targetDir); err != nil {
			http.Error(w, "Taşıma hatası: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// 2. Sonra aynı hash'e sahip diğerlerini sil
		if err := mediaService.DeleteOtherDuplicates(id, hashID); err != nil {
			http.Error(w, "Diğerlerini silme hatası: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/add-tag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		hashID := r.URL.Query().Get("hash_id")
		tag := r.URL.Query().Get("tag")
		if hashID == "" || tag == "" {
			http.Error(w, "Geçersiz parametreler", http.StatusBadRequest)
			return
		}

		tags := strings.Split(tag, ",")
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			err := mediaRepo.AddTag(hashID, t)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/remove-tag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		hashID := r.URL.Query().Get("hash_id")
		tag := r.URL.Query().Get("tag")
		if hashID == "" || tag == "" {
			http.Error(w, "Geçersiz parametreler", http.StatusBadRequest)
			return
		}
		err := mediaRepo.RemoveTag(hashID, tag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/duplicates", func(w http.ResponseWriter, r *http.Request) {
		hashID := r.URL.Query().Get("hash_id")

		data := struct {
			Duplicates      []map[string]interface{}
			ScanRecords     []repository.MediaScanRecord
			IndexSuggestion struct {
				ImagePath string `yaml:"image_path"`
				VideoPath string `yaml:"video_path"`
			}
			DefaultHashtag string
		}{
			IndexSuggestion: cfg.IndexSuggestion,
			DefaultHashtag:  cfg.Hashtag.DefaultHashtag,
		}

		var err error
		data.Duplicates, err = mediaRepo.GetTopDuplicateHashes(30)
		if err != nil {
			log.Println("Error getting duplicates:", err)
			http.Error(w, err.Error(), 500)
			return
		}

		if hashID != "" {
			data.ScanRecords, err = mediaRepo.GetScansByHashID(hashID)
			if err != nil {
				log.Println("Error getting scans:", err)
				http.Error(w, err.Error(), 500)
				return
			}
		}

		tmpl, err := template.New("duplicates.html").Funcs(template.FuncMap{
			"isImage": isImage,
			"isVideo": isVideo,
		}).ParseFiles("web/duplicates.html")
		if err != nil {
			log.Println("Error parsing template:", err)
			http.Error(w, err.Error(), 500)
			return
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			log.Println("Error executing template:", err)
		}
	})

	http.HandleFunc("/untagged", func(w http.ResponseWriter, r *http.Request) {
		hashID := r.URL.Query().Get("hash_id")

		data := struct {
			Untagged        []map[string]interface{}
			ScanRecords     []repository.MediaScanRecord
			IndexSuggestion struct {
				ImagePath string `yaml:"image_path"`
				VideoPath string `yaml:"video_path"`
			}
			DefaultHashtag string
		}{
			IndexSuggestion: cfg.IndexSuggestion,
			DefaultHashtag:  cfg.Hashtag.DefaultHashtag,
		}

		var err error
		data.Untagged, err = mediaRepo.GetUntaggedHashes(30)
		if err != nil {
			log.Println("Error getting untagged hashes:", err)
			http.Error(w, err.Error(), 500)
			return
		}

		if hashID != "" {
			data.ScanRecords, err = mediaRepo.GetScansByHashID(hashID)
			if err != nil {
				log.Println("Error getting scans:", err)
				http.Error(w, err.Error(), 500)
				return
			}
		}

		tmpl, err := template.New("untagged.html").Funcs(template.FuncMap{
			"isImage": isImage,
			"isVideo": isVideo,
		}).ParseFiles("web/untagged.html")
		if err != nil {
			log.Println("Error parsing template:", err)
			http.Error(w, err.Error(), 500)
			return
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			log.Println("Error executing template:", err)
		}
	})

	http.HandleFunc("/scan", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("web/scan.html")
		if err != nil {
			log.Println("Error parsing template:", err)
			http.Error(w, err.Error(), 500)
			return
		}
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/serve-file", func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Clean(r.URL.Query().Get("path"))

		// Dosyanın varlığını kontrol et
		if info, err := os.Stat(filePath); err != nil || info.IsDir() {
			log.Printf("Dosya erişim hatası: %s - Hata: %v", filePath, err)
			http.Error(w, "Dosya bulunamadı veya erişilemez", http.StatusNotFound)
			return
		}

		// http.ServeFile, HTTP Range Requests desteğini otomatik olarak sağlar.
		// Bu sayede video oynatıcısında ileri-geri sarma özelliği çalışır.

		// Windows sistemlerde bazen MIME tipi yanlış belirlenebilir, manuel set edelim
		if strings.HasSuffix(strings.ToLower(filePath), ".mp4") {
			w.Header().Set("Content-Type", "video/mp4")
		}

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filePath)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			fullPath := filepath.Join("web", r.URL.Path)
			if _, err := os.Stat(fullPath); err == nil {
				http.ServeFile(w, r, fullPath)
				return
			}
		}

		// Filtreleme ve Paging parametrelerini al
		selectedTags := r.URL.Query()["tags"]
		fileType := r.URL.Query().Get("file_type")
		// Açılışta (parametre yoksa) resimleri varsayılan yap
		if fileType == "" {
			fileType = "image"
		}

		pageStr := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageStr)
		if page < 1 {
			page = 1
		}
		// Videolar ağır olduğu için sayfa başına daha az kayıt göster
		limit := 45
		if fileType == "video" {
			limit = 10
		}
		offset := (page - 1) * limit

		allTags, _ := mediaRepo.GetAllTags(r.Context(), selectedTags, fileType)
		mediaList, total, err := mediaRepo.GetFilteredMedia(r.Context(), selectedTags, fileType, limit, offset)

		data := struct {
			AllTags          []repository.TagCount
			SelectedTags     []string
			SelectedFileType string
			MediaList        []repository.MediaScanRecord
			CurrentPage      int
			TotalPages       int
			NextPage         int
			PrevPage         int
			IndexSuggestion  struct {
				ImagePath string `yaml:"image_path"`
				VideoPath string `yaml:"video_path"`
			}
			DefaultHashtag string
		}{
			AllTags:          allTags,
			SelectedTags:     selectedTags,
			SelectedFileType: fileType,
			MediaList:        mediaList,
			CurrentPage:      page,
			TotalPages:       (total + limit - 1) / limit,
			NextPage:         page + 1,
			PrevPage:         page - 1,
			IndexSuggestion:  cfg.IndexSuggestion,
			DefaultHashtag:   cfg.Hashtag.DefaultHashtag,
		}

		tmpl, err := template.New("index.html").Funcs(template.FuncMap{
			"isImage": isImage,
			"isVideo": isVideo,
			"contains": func(slice []string, item string) bool {
				for _, s := range slice {
					if s == item {
						return true
					}
				}
				return false
			},
		}).ParseFiles("web/index.html")

		if err != nil {
			log.Println("Error parsing template:", err)
			http.Error(w, err.Error(), 500)
			return
		}
		tmpl.Execute(w, data)
	})

	url := "http://localhost:8080"
	fmt.Printf("Sunucu başlatıldı: %s\n", url)

	// Tarayıcıyı goroutine içinde otomatik olarak aç
	go openBrowser(url)

	http.ListenAndServe(":8080", nil)
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("desteklenmeyen platform")
	}
	if err != nil {
		log.Printf("Tarayıcı otomatik açılamadı: %v", err)
	}
}
