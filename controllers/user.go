package controllers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"mate/models"
)

const (
	leaderboardCacheKey    = "leaderboard:weekly"
	leaderboardEmptyMember = "__leaderboard_empty__"
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
			"name":        item.ItemName,
			"points":      item.ItemPoints,
			"image":       item.ItemImage,
			"redeemed_at": item.RedeemedAt.Format("2006-01-02"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nickname":            user.Nickname,
		"avatar":              user.Avatar,
		"current_points":      user.CurrentPoints,
		"total_earned_points": user.TotalEarnedPoints,
		"redeemed_items":      redeemedList,
		"rate":                user.ExchangeRate,
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
	ctx := context.Background()
	type leaderboardRow struct {
		Nickname    string  `json:"nickname"`
		Avatar      string  `json:"avatar"`
		TotalPoints float64 `json:"total_points"`
	}

	if exists, err := models.RDB.Exists(ctx, leaderboardCacheKey).Result(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis 查询异常"})
		return
	} else if exists > 0 {
		members, err := models.RDB.ZRevRangeWithScores(ctx, leaderboardCacheKey, 0, -1).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis 查询异常"})
			return
		}
		if len(members) == 0 {
			c.JSON(http.StatusOK, []leaderboardRow{})
			return
		}

		userIDs := make([]string, 0, len(members))
		for _, member := range members {
			if member.Member == leaderboardEmptyMember {
				continue
			}
			idStr, ok := member.Member.(string)
			if !ok {
				continue
			}
			userIDs = append(userIDs, idStr)
		}
		if len(userIDs) == 0 {
			c.JSON(http.StatusOK, []leaderboardRow{})
			return
		}

		var users []models.User
		if err := models.DB.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
			return
		}
		userMap := make(map[string]models.User, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}

		results := make([]leaderboardRow, 0, len(members))
		for _, member := range members {
			idStr, ok := member.Member.(string)
			if !ok || idStr == leaderboardEmptyMember {
				continue
			}
			user, ok := userMap[idStr]
			if !ok {
				continue
			}
			results = append(results, leaderboardRow{
				Nickname:    user.Nickname,
				Avatar:      user.Avatar,
				TotalPoints: member.Score,
			})
		}
		c.JSON(http.StatusOK, results)
		return
	}

	type rawLeaderboardRow struct {
		UserID      string  `json:"user_id"`
		Nickname    string  `json:"nickname"`
		Avatar      string  `json:"avatar"`
		TotalPoints float64 `json:"total_points"`
	}
	var rawRows []rawLeaderboardRow

	models.DB.Raw(`
		SELECT users.id as user_id, users.nickname, users.avatar, SUM(tasks.points) as total_points
		FROM users
		JOIN tasks ON users.id = tasks.user_id
		WHERE tasks.status = ?
		GROUP BY users.id, users.nickname, users.avatar
		ORDER BY total_points DESC
	`, "completed").Scan(&rawRows)

	if len(rawRows) == 0 {
		models.RDB.ZAdd(ctx, leaderboardCacheKey, redis.Z{Score: 0, Member: leaderboardEmptyMember})
		models.RDB.Expire(ctx, leaderboardCacheKey, 24*time.Hour)
		c.JSON(http.StatusOK, []leaderboardRow{})
		return
	}

	zMembers := make([]redis.Z, 0, len(rawRows))
	results := make([]leaderboardRow, 0, len(rawRows))
	for _, row := range rawRows {
		zMembers = append(zMembers, redis.Z{Score: row.TotalPoints, Member: row.UserID})
		results = append(results, leaderboardRow{
			Nickname:    row.Nickname,
			Avatar:      row.Avatar,
			TotalPoints: row.TotalPoints,
		})
	}
	models.RDB.ZAdd(ctx, leaderboardCacheKey, zMembers...)
	models.RDB.Expire(ctx, leaderboardCacheKey, 24*time.Hour)
	c.JSON(http.StatusOK, results)
}
