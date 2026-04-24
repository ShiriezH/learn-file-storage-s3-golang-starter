package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	fmt.Println("UPLOAD HIT")

	// Prevent server crash (very important for your error)
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("PANIC:", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	const maxMemory = 10 << 20

	// Debug path
	fmt.Println("PATH:", r.URL.Path)

	// Parse multipart form
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse multipart form", err)
		return
	}

	// Get file
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get file", err)
		return
	}
	defer file.Close()

	// Get content type
	mediaType := header.Header.Get("Content-Type")

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read file", err)
		return
	}

	// Get video ID from URL
	videoIDStr := r.PathValue("videoID")
	if videoIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "Missing video ID", nil)
		return
	}

	fmt.Println("videoIDStr:", videoIDStr)

	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	// Get video from DB
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}

	// Save thumbnail in memory
	videoThumbnails[videoID] = thumbnail{
		data:      data,
		mediaType: mediaType,
	}

	// Create thumbnail URL
	url := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoID.String())
	video.ThumbnailURL = &url

	// Update DB
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	// Success response
	respondWithJSON(w, http.StatusOK, video)
}
