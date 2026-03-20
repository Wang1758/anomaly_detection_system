package handler

import (
	"path/filepath"
	"strings"

	"anomaly_detection_system/backend/internal/models"
)

// resolveSampleImagePath returns the filesystem path to a sample's JPEG.
// Supports legacy rows that stored either a basename or an absolute path.
func resolveSampleImagePath(dataDir string, s models.Sample) string {
	if s.ImagePath == "" {
		return ""
	}
	p := s.ImagePath
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dataDir, "images", filepath.Clean(filepath.Base(p)))
}

// safeImageFilename returns a single path segment for use under data/images.
func safeImageFilename(param string) (string, bool) {
	name := filepath.Base(strings.TrimSpace(param))
	if name == "" || name == "." || name == ".." {
		return "", false
	}
	return name, true
}
