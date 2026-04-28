package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

type ffprobeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var result ffprobeOutput
	err = json.Unmarshal(out.Bytes(), &result)
	if err != nil {
		return "", err
	}

	if len(result.Streams) == 0 {
		return "other", nil
	}

	width := result.Streams[0].Width
	height := result.Streams[0].Height

	// Determine aspect ratio category
	if width > height {
		return "landscape", nil
	} else if height > width {
		return "portrait", nil
	}

	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := filePath + ".processing.mp4"

	cmd := exec.Command(
		"ffmpeg",
		"-i", filePath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		outputPath,
	)

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputPath, nil
}