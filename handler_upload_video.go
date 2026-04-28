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
	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	// Parse multipart form
	err := r.ParseMultipartForm(1 << 30)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not parse form", err)
		return
	}

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
	if err != nil || (mediaType != "video/mp4" && mediaType != "application/octet-stream") {
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

	// Detect aspect ratio
	aspect, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not detect aspect ratio", err)
		return
	}

	// Generate random key
	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "random failed", err)
		return
	}

	// Prefix path (landscape / portrait / other)
	fileKey := fmt.Sprintf("%s/%s.mp4", aspect, hex.EncodeToString(randomBytes))

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

	// Build URL
	videoURL := fmt.Sprintf(
		"https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket,
		cfg.s3Region,
		fileKey,
	)

	fmt.Println("UPLOADING VIDEO:", video.ID, videoURL)

	// Update video record with URL
	err = cfg.db.UpdateVideoURL(video.ID, videoURL)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "db update failed", err)
		return
	}

	fmt.Println("UPDATED DB SUCCESSFULLY")

	video.VideoURL = &videoURL

	respondWithJSON(w, http.StatusOK, video)
}
