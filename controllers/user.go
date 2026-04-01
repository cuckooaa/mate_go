package controllers

import (
	"net/http"
	"strings"
	"github.com/gin-gonic/gin"
	"mate/models"
)

// GetUserInfo 获取用户信息和兑换记录
func GetUserInfo(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var redeemedItems []models.RedeemedItem
	models.DB.Where("user_id = ?", userID).Order("redeemed_at DESC").Find(&redeemedItems)

	redeemedList := make([]gin.H, 0, len(redeemedItems))
	for _, item := range redeemedItems {
		redeemedList = append(redeemedList, gin.H{
			"name":       item.ItemName,
			"points":     item.ItemPoints,
			"image":      item.ItemImage,
			"redeemed_at": item.RedeemedAt.Format("2006-01-02"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nickname":           user.Nickname,
		"avatar":             user.Avatar,
		"current_points":     user.CurrentPoints,
		"total_earned_points": user.TotalEarnedPoints,
		"redeemed_items":     redeemedList,
		"rate":               user.ExchangeRate,
	})
}

// UpdateNickname 更新用户昵称
func UpdateNickname(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	var req struct {
		NewNickname string `json:"newNickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.NewNickname) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新昵称不能为空"})
		return
	}

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	user.Nickname = req.NewNickname
	if err := models.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "昵称更新成功"})
}

// UpdateAvatar 更新用户头像
func UpdateAvatar(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	var req struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AvatarURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像 URL 不能为空"})
		return
	}

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 删除原头像（如果不是默认头像）
	if user.Avatar != "" && strings.Contains(user.Avatar, "/uploads/") {
		_ = DeleteLocalFile(user.Avatar)
	}

	user.Avatar = req.AvatarURL
	if err := models.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "头像更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "头像更新成功"})
}

// GetLeaderboard 获取排行榜（按积分总和排序，含头像和昵称）
func GetLeaderboard(c *gin.Context) {
	// 集成原生 SQL 或 GORM API
	type leaderboardRow struct {
		Nickname    string   `json:"nickname"`
		Avatar      string   `json:"avatar"`
		TotalPoints float64  `json:"total_points"`
	}
	var results []leaderboardRow

	// 用 GORM 的 Raw SQL
	models.DB.Raw(`
		SELECT users.nickname, users.avatar, SUM(tasks.points) as total_points
		FROM users
		JOIN tasks ON users.id = tasks.user_id
		WHERE tasks.status = ?
		GROUP BY users.nickname, users.avatar
		ORDER BY total_points DESC
	`, "completed").Scan(&results)

	c.JSON(http.StatusOK, results)
}


