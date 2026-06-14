package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type MediaScanRecord struct {
	ID               int64
	MediaHashID      string
	FilePath         string
	FileName         string
	FileSize         int64
	FileLastModified time.Time
	FileFirstCreate  time.Time
	Tags             []string
}

type TagCount struct {
	Name  string
	Count int
}

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// Hash'i ekler veya varsa günceller, ID'sini döner
func (r *MediaRepository) UpsertMediaHash(hash string) (string, error) {
	var mediaHashID string
	query := `
		INSERT INTO media_hash (media_hash) 
		VALUES ($1) 
		ON CONFLICT (media_hash) DO UPDATE SET update_date = clock_timestamp()
		RETURNING media_hash_id`

	err := r.db.QueryRow(query, hash).Scan(&mediaHashID)
	return mediaHashID, err
}

// Tarama kaydını ekler veya dosya yolu aynıysa hash'i günceller
func (r *MediaRepository) UpsertMediaScan(hashID string, record MediaScanRecord) error {
	query := `
		INSERT INTO media_scan (
			media_hash_id, file_path, file_name, file_size, 
			file_last_modified_time, file_first_create_time
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (file_path) DO UPDATE SET 
			media_hash_id = EXCLUDED.media_hash_id,
			update_date = clock_timestamp()`

	_, err := r.db.Exec(query,
		hashID, record.FilePath, record.FileName,
		record.FileSize, record.FileLastModified, record.FileFirstCreate,
	)
	return err
}

// En çok tekrar eden 20 hash'i getirir
func (r *MediaRepository) GetTopDuplicateHashes(limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT mh.media_hash, mh.media_hash_id, COUNT(ms.media_hash_id) as repeat_count
		FROM media_hash mh
		JOIN media_scan ms ON mh.media_hash_id = ms.media_hash_id
		GROUP BY mh.media_hash_id, mh.media_hash
		HAVING COUNT(ms.media_hash_id) > 1
		ORDER BY repeat_count DESC,mh.media_hash_id desc
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var hash, id string
		var count int
		rows.Scan(&hash, &id, &count)
		results = append(results, map[string]interface{}{
			"Hash":  hash,
			"ID":    id,
			"Count": count,
		})
	}
	return results, nil
}

// GetUntaggedHashes etiketlenmemiş tüm hash'leri getirir
func (r *MediaRepository) GetUntaggedHashes(limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT mh.media_hash, mh.media_hash_id, COUNT(ms.id) as file_count
		FROM media_hash mh
		JOIN media_scan ms ON mh.media_hash_id = ms.media_hash_id
		LEFT JOIN media_scan_hash_tag msht ON mh.media_hash_id = msht.media_hash_id
		WHERE msht.media_hash_id IS NULL
		GROUP BY mh.media_hash_id, mh.media_hash
		ORDER BY mh.media_hash_id DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var hash, id string
		var count int
		err := rows.Scan(&hash, &id, &count)
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"Hash":  hash,
			"ID":    id,
			"Count": count,
		})
	}
	return results, nil
}

// Belirli bir hash_id'ye ait tüm dosyaları getirir
func (r *MediaRepository) GetScansByHashID(hashID string) ([]MediaScanRecord, error) {
	var records []MediaScanRecord
	query := `
		SELECT ms.id, ms.media_hash_id, ms.file_path, ms.file_name, ms.file_size, 
		       ms.file_last_modified_time, ms.file_first_create_time,
		       COALESCE(array_agg(mht.tag_name) FILTER (WHERE mht.tag_name IS NOT NULL), '{}') as tags
		FROM media_scan ms
		LEFT JOIN media_scan_hash_tag msht ON ms.media_hash_id = msht.media_hash_id
		LEFT JOIN media_hash_tag mht ON msht.media_hash_tag_id = mht.id
		WHERE ms.media_hash_id = $1
		GROUP BY ms.id`

	rows, err := r.db.Query(query, hashID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rec MediaScanRecord
		err := rows.Scan(&rec.ID, &rec.MediaHashID, &rec.FilePath, &rec.FileName, &rec.FileSize, &rec.FileLastModified, &rec.FileFirstCreate, pq.Array(&rec.Tags))
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

// Kaydı siler
func (r *MediaRepository) DeleteMediaScan(id int64) error {
	_, err := r.db.Exec("DELETE FROM media_scan WHERE id = $1", id)
	return err
}

// Tekil kaydı getirir (Dosya yolunu bulmak için)
func (r *MediaRepository) GetScanByID(id int64) (MediaScanRecord, error) {
	var record MediaScanRecord
	query := `SELECT id, file_path, file_name FROM media_scan WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(&record.ID, &record.FilePath, &record.FileName)
	return record, err
}

func (r *MediaRepository) UpdateMediaPath(scanID int64, newPath string, newName string) error {
	query := `UPDATE media_scan SET file_path = $1, file_name = $2, update_date = clock_timestamp() WHERE id = $3`
	_, err := r.db.Exec(query, newPath, newName, scanID)
	return err
}

func (r *MediaRepository) GetOtherDuplicateIDs(keepID int64, hashID string) ([]int64, error) {
	query := `SELECT id FROM media_scan WHERE media_hash_id = $1 AND id != $2`
	rows, err := r.db.Query(query, hashID, keepID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Belirli bir hash_id'ye etiket ekler veya var olan etikete sahip değilse oluşturur, ilişkiyi kurar
func (r *MediaRepository) AddTag(hashID string, tagName string) error {
	var tagID int
	// Etiket yoksa oluştur, varsa ID'sini al
	err := r.db.QueryRow(`
		INSERT INTO media_hash_tag (tag_name) 
		VALUES ($1) 
		ON CONFLICT (tag_name) DO UPDATE SET tag_name = EXCLUDED.tag_name
		RETURNING id`, tagName).Scan(&tagID)
	if err != nil {
		return err
	}

	// İlişkiyi kur
	_, err = r.db.Exec(`
		INSERT INTO media_scan_hash_tag (media_hash_id, media_hash_tag_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, hashID, tagID)
	return err
}

// Belirli bir hash_id'ye ait etiketi kaldırır
func (r *MediaRepository) RemoveTag(hashID string, tagName string) error {
	query := `
		DELETE FROM media_scan_hash_tag 
		WHERE media_hash_id = $1 
		AND media_hash_tag_id = (SELECT id FROM media_hash_tag WHERE tag_name = $2)`
	_, err := r.db.Exec(query, hashID, tagName)
	return err
}

// Belirli bir hash_id'ye ait tüm etiketleri getirir
func (r *MediaRepository) GetAllTags(ctx context.Context, selectedTags []string, fileType string) ([]TagCount, error) {
	whereClause := ""
	var args []interface{}
	if len(selectedTags) > 0 {
		whereClause = `WHERE ms.media_hash_id IN (
			SELECT msht2.media_hash_id FROM media_scan_hash_tag msht2 
			JOIN media_hash_tag mht2 ON msht2.media_hash_tag_id = mht2.id 
			WHERE mht2.tag_name = ANY($1)
			GROUP BY msht2.media_hash_id
			HAVING COUNT(DISTINCT mht2.tag_name) = array_length($1, 1)
		)`
		args = append(args, pq.Array(selectedTags))
	}

	if fileType != "" && fileType != "all" {
		if whereClause == "" {
			whereClause = "WHERE "
		} else {
			whereClause += " AND "
		}
		if fileType == "image" {
			whereClause += "LOWER(RIGHT(ms.file_name, 4)) IN ('.jpg', 'jpeg', '.png', '.gif', 'webp')"
		} else if fileType == "video" {
			whereClause += "LOWER(RIGHT(ms.file_name, 4)) IN ('.mp4', '.mov', '.avi', '.mkv')"
		}
	}

	query := fmt.Sprintf(`
		SELECT mht.tag_name, COUNT(DISTINCT ms.id) as tag_count
		FROM media_hash_tag mht
		JOIN media_scan_hash_tag msht ON mht.id = msht.media_hash_tag_id
		JOIN media_scan ms ON msht.media_hash_id = ms.media_hash_id
		%s
		GROUP BY mht.id, mht.tag_name
		ORDER BY mht.tag_name ASC`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []TagCount
	for rows.Next() {
		var tc TagCount
		rows.Scan(&tc.Name, &tc.Count)
		tags = append(tags, tc)
	}
	return tags, nil
}

// Medya kayıtlarını seçilen etiketlere ve dosya tipine göre filtreler, sayfalama yapar
func (r *MediaRepository) GetFilteredMedia(ctx context.Context, tags []string, fileType string, limit, offset int) ([]MediaScanRecord, int, error) {
	var records []MediaScanRecord
	var totalCount int

	whereClause := ""
	var args []interface{}
	if len(tags) > 0 {
		// Tüm seçilen etiketlere sahip olanları getirir (AND mantığı)
		whereClause = `WHERE ms.media_hash_id IN (
			SELECT msht2.media_hash_id FROM media_scan_hash_tag msht2 
			JOIN media_hash_tag mht2 ON msht2.media_hash_tag_id = mht2.id 
			WHERE mht2.tag_name = ANY($1)
			GROUP BY msht2.media_hash_id
			HAVING COUNT(DISTINCT mht2.tag_name) = array_length($1, 1)
		)`
		args = append(args, pq.Array(tags))
	}

	// Dosya tipi filtresi
	if fileType != "" && fileType != "all" {
		if whereClause == "" {
			whereClause = "WHERE "
		} else {
			whereClause += " AND "
		}
		if fileType == "image" {
			whereClause += "LOWER(RIGHT(ms.file_name, 4)) IN ('.jpg', 'jpeg', '.png', '.gif', 'webp')"
		} else if fileType == "video" {
			whereClause += "LOWER(RIGHT(ms.file_name, 4)) IN ('.mp4', '.mov', '.avi', '.mkv')"
		}
	}

	// Toplam sayıyı al
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT ms.id) FROM media_scan ms %s", whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Verileri çek
	query := fmt.Sprintf(`
		SELECT ms.id, ms.media_hash_id, ms.file_path, ms.file_name, ms.file_size, 
		       ms.file_last_modified_time, ms.file_first_create_time,
		       COALESCE(array_agg(mht.tag_name) FILTER (WHERE mht.tag_name IS NOT NULL), '{}') as tags
		FROM media_scan ms
		LEFT JOIN media_scan_hash_tag msht ON ms.media_hash_id = msht.media_hash_id
		LEFT JOIN media_hash_tag mht ON msht.media_hash_tag_id = mht.id
		%s
		GROUP BY ms.id
		ORDER BY ms.file_name ASC
		LIMIT $%d OFFSET $%d`, whereClause, len(args)+1, len(args)+2)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var rec MediaScanRecord
		err := rows.Scan(&rec.ID, &rec.MediaHashID, &rec.FilePath, &rec.FileName, &rec.FileSize, &rec.FileLastModified, &rec.FileFirstCreate, pq.Array(&rec.Tags))
		if err != nil {
			return nil, 0, err
		}
		records = append(records, rec)
	}
	return records, totalCount, nil
}
