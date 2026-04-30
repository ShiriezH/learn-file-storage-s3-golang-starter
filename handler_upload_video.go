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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	err := r.ParseMultipartForm(1 << 30)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not parse form", err)
		return
	}

	videoIDStr := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid video ID", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not read file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil || (mediaType != "video/mp4" && mediaType != "application/octet-stream") {
		respondWithError(w, http.StatusBadRequest, "only mp4 allowed", nil)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "temp file error", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "copy failed", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "seek failed", err)
		return
	}

	aspect, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "aspect ratio failed", err)
		return
	}

	processedPath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "processing failed", err)
		return
	}
	defer os.Remove(processedPath)

	processedFile, err := os.Open(processedPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "open processed failed", err)
		return
	}
	defer processedFile.Close()

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "random failed", err)
		return
	}

	fileKey := fmt.Sprintf("%s/%s.mp4", aspect, hex.EncodeToString(randomBytes))

	_, err = cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        processedFile,
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "upload failed", err)
		return
	}

	// In a production app, you'd want to invalidate the CloudFront cache for this file here.
	videoURL := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, fileKey)

	err = cfg.db.UpdateVideoURL(video.ID, videoURL)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "db update failed", err)
		return
	}

	video.VideoURL = &videoURL

	respondWithJSON(w, http.StatusOK, video)
}
