package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var imageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

func (a *App) handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	extension := strings.ToLower(filepath.Ext(file.Filename))
	if !imageExtensions[extension] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only image files are allowed"})
		return
	}

	if err := os.MkdirAll("uploads", 0755); err != nil {
		a.logger.Error("failed to create uploads directory", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare file upload"})
		return
	}

	filename := uuid.NewString() + extension
	path := filepath.Join("uploads", filename)
	if err := c.SaveUploadedFile(file, path); err != nil {
		a.logger.Error("failed to save uploaded file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"path": "/uploads/" + filename})
}
