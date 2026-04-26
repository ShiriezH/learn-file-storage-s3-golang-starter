package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/joho/godotenv"
)

type apiConfig struct {
	db               database.Client
	jwtSecret        string
	platform         string
	filepathRoot     string
	assetsRoot       string
	s3Bucket         string
	s3Region         string
	s3CfDistribution string
	port             string
	s3Client         *s3.Client
}

func main() {
	godotenv.Load(".env")

	pathToDB := os.Getenv("DB_PATH")
	if pathToDB == "" {
		log.Fatal("DB_PATH must be set")
	}

	dbClient, err := database.NewClient(pathToDB)
	if err != nil {
		log.Fatalf("Couldn't connect to database: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	platform := os.Getenv("PLATFORM")
	filepathRoot := os.Getenv("FILEPATH_ROOT")
	assetsRoot := os.Getenv("ASSETS_ROOT")
	s3Bucket := os.Getenv("S3_BUCKET")
	s3Region := os.Getenv("S3_REGION")
	s3CfDistribution := os.Getenv("S3_CF_DISTRO")
	port := os.Getenv("PORT")

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(s3Region),
	)
	if err != nil {
		log.Fatalf("unable to load AWS config: %v", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)

	cfg := apiConfig{
		db:               dbClient,
		jwtSecret:        jwtSecret,
		platform:         platform,
		filepathRoot:     filepathRoot,
		assetsRoot:       assetsRoot,
		s3Bucket:         s3Bucket,
		s3Region:         s3Region,
		s3CfDistribution: s3CfDistribution,
		port:             port,
		s3Client:         s3Client,
	}

	err = cfg.ensureAssetsDir()
	if err != nil {
		log.Fatalf("Couldn't create assets directory: %v", err)
	}

	mux := http.NewServeMux()

	// Static
	appHandler := http.StripPrefix("/app/", http.FileServer(http.Dir(filepathRoot)))
	mux.Handle("/app/", appHandler)

	assetsHandler := http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsRoot)))
	mux.Handle("/assets/", noCacheMiddleware(assetsHandler))

	// Auth
	mux.HandleFunc("POST /api/login", cfg.handlerLogin)
	mux.HandleFunc("POST /api/refresh", cfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", cfg.handlerRevoke)

	// Users
	mux.HandleFunc("POST /api/users", cfg.handlerUsersCreate)

	// Videos
	mux.HandleFunc("POST /api/videos", cfg.handlerVideoMetaCreate)
	mux.HandleFunc("POST /api/videos/{videoID}/thumbnail", cfg.handlerUploadThumbnail)
	mux.HandleFunc("POST /api/videos/{videoID}/video", cfg.handlerUploadVideo)
	mux.HandleFunc("GET /api/videos", cfg.handlerVideosRetrieve)
	mux.HandleFunc("GET /api/videos/{videoID}", cfg.handlerVideoGet)
	mux.HandleFunc("DELETE /api/videos/{videoID}", cfg.handlerVideoMetaDelete)

	// Admin
	mux.HandleFunc("POST /admin/reset", cfg.handlerReset)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving on: http://localhost:%s/app/\n", port)
	log.Fatal(srv.ListenAndServe())
}
