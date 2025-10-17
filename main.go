package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client
var db *sql.DB
var bucket = "files"

func main() {
	var err error

	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	log.Println("Connecting to PostgreSQL:", connStr)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Connection error:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Ping error:", err)
	}

	_, err = db.Exec("CREATE DATABASE filedb")
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		log.Fatal("Create DB error:", err)
	}

	db.Close()

	connStr = "host=localhost port=5432 user=postgres password=postgres dbname=filedb sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Connection to filedb error:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			original_name VARCHAR(255) NOT NULL,
			size BIGINT NOT NULL,
			content_type VARCHAR(100),
			uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal("Error creating table:", err)
	}

	minioClient, err = minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	exists, _ := minioClient.BucketExists(ctx, bucket)
	if !exists {
		minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}

	http.HandleFunc("/upload", upload)
	http.HandleFunc("/download", download)
	http.HandleFunc("/files", list)
	http.HandleFunc("/delete", delete)

	log.Println("Server: http://localhost:8080")
	log.Println("MinIO Console: http://localhost:9001")
	log.Println("PostgreSQL: localhost:5432")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func upload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file not found", 400)
		return
	}
	defer file.Close()
	id := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	name := id + ext

	_, err = minioClient.PutObject(context.Background(), bucket, name, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	_, err = db.Exec(
		"INSERT INTO files (id, name, original_name, size, content_type) VALUES ($1, $2, $3, $4, $5)",
		id, name, header.Filename, header.Size, header.Header.Get("Content-Type"),
	)
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "name": name, "original_name": header.Filename})
}
func download(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", 400)
		return
	}

	exts := []string{"", ".jpg", ".png", ".pdf", ".txt", ".zip", ".docx", ".mp4"}
	var obj *minio.Object
	var name string

	for _, ext := range exts {
		n := id + ext
		var err error
		obj, err = minioClient.GetObject(context.Background(), bucket, n, minio.GetObjectOptions{})
		if err == nil {
			if _, err = obj.Stat(); err == nil {
				name = n
				break
			}
		}
	}

	if name == "" {
		http.Error(w, "not found", 404)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	io.Copy(w, obj)
}

func list(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, original_name, size, content_type, uploaded_at FROM files ORDER BY uploaded_at DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var files []map[string]interface{}
	for rows.Next() {
		var id, name, originalName, contentType string
		var size int64
		var uploadedAt time.Time
		err := rows.Scan(&id, &name, &originalName, &size, &contentType, &uploadedAt)
		if err != nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"id":            id,
			"name":          name,
			"original_name": originalName,
			"size":          size,
			"content_type":  contentType,
			"uploaded_at":   uploadedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func delete(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}

	err := minioClient.RemoveObject(context.Background(), bucket, name, minio.RemoveObjectOptions{})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	_, err = db.Exec("DELETE FROM files WHERE name = $1", name)
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
