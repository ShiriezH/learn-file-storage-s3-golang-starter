package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 1GB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	// Parse video ID
	videoIDStr := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid video ID", err)
		return
	}

	// Get video from DB
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not read file", err)
		return
	}
	defer file.Close()

	// Validate MIME type
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil || mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "only mp4 allowed", nil)
		return
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", "tubely-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "temp file error", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy file to temp
	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "copy failed", err)
		return
	}

	// Reset pointer
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "seek failed", err)
		return
	}

	// Generate random key
	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "random failed", err)
		return
	}

	fileKey := hex.EncodeToString(randomBytes) + ".mp4"

	// Upload to S3
	_, err = cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        tempFile,
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "upload failed", err)
		return
	}

	// Build S3 URL
	videoURL := fmt.Sprintf(
		"https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket,
		cfg.s3Region,
		fileKey,
	)

	video.VideoURL = &videoURL

	// Update DB
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "db update failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
