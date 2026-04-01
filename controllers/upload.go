package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 允许上传的图片扩展名
var allowedExtensions = map[string]struct{}{
	"png":  {},
	"jpg":  {},
	"jpeg": {},
	"gif":  {},
	"webp": {},
	"heic": {},
	"heif": {},
}

// UploadFile Gin处理函数：接收图片上传
func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到文件"})
		return
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file.Filename), "."))
	if _, ok := allowedExtensions[ext]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型"})
		return
	}

	// 生成唯一文件名
	filename := uuid.New().String() + "." + ext
	uploadDir := "./uploads/"
	dstPath := filepath.Join(uploadDir, filename)

	// 确保目录存在
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建上传目录失败"})
		return
	}

	if err := c.SaveUploadedFile(file, dstPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	imageURL := "/uploads/" + filename
	c.JSON(http.StatusCreated, gin.H{
		"message": "文件上传成功",
		"url":     imageURL,
	})
}

// DeleteLocalFile 删除本地uploads目录下的文件辅助函数
func DeleteLocalFile(fileURL string) error {
	if fileURL == "" {
		return nil
	}

	// 只接受 /uploads/filename 这类路径
	fileName := filepath.Base(fileURL)
	if fileName == "" || fileName == "." || fileName == "/" {
		return nil // 或可返回错误
	}
	filePath := filepath.Join("./uploads/", fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 文件不存在不报错
	}
	return os.Remove(filePath)
}
