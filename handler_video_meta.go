package main

import (
	"encoding/json"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerVideoMetaCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		database.CreateVideoParams
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	params := parameters{}
	err = json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "decode failed", err)
		return
	}

	params.UserID = userID

	video, err := cfg.db.CreateVideo(params.CreateVideoParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "create failed", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)
}

func (cfg *apiConfig) handlerVideoMetaDelete(w http.ResponseWriter, r *http.Request) {
	videoID, _ := uuid.Parse(r.PathValue("videoID"))

	token, _ := auth.GetBearerToken(r.Header)
	userID, _ := auth.ValidateJWT(token, cfg.jwtSecret)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "not found", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusForbidden, "not allowed", nil)
		return
	}

	err = cfg.db.DeleteVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "delete failed", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerVideoGet(w http.ResponseWriter, r *http.Request) {
	videoID, _ := uuid.Parse(r.PathValue("videoID"))

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "not found", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func (cfg *apiConfig) handlerVideosRetrieve(w http.ResponseWriter, r *http.Request) {
	token, _ := auth.GetBearerToken(r.Header)
	userID, _ := auth.ValidateJWT(token, cfg.jwtSecret)

	videos, err := cfg.db.GetVideos(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "fetch failed", err)
		return
	}

	filtered := []database.Video{}

	for _, v := range videos {
		if v.VideoURL != nil {
			filtered = append(filtered, v)
		}
	}

	respondWithJSON(w, http.StatusOK, filtered)
}
