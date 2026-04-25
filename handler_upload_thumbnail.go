package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 20

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

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read file", err)
		return
	}

	// Get video ID
	videoIDStr := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	// Convert to base64
	encoded := base64.StdEncoding.EncodeToString(data)

	// Build data URL
	mediaType := header.Header.Get("Content-Type")
	thumbnailURL := fmt.Sprintf("data:%s;base64,%s", mediaType, encoded)

	// Save to DB
	err = cfg.db.UpdateVideoThumbnail(videoID, thumbnailURL)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update thumbnail", err)
		return
	}

	// Success
	w.WriteHeader(http.StatusOK)
}
