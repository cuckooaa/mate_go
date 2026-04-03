package controllers

import (
	"math"
	"net/http"
	"strings"
	"strconv"
	"github.com/gin-gonic/gin"
	"mate/models"
	"context"
	"time"
	"encoding/json"
	"github.com/redis/go-redis/v9"
)

// GetPublicShopItems 查询公共商品，使用 Redis 缓存
func GetPublicShopItems(c *gin.Context) {
	ctx := context.Background()
	const redisKey = "shop:public_items"
	// 先查 Redis
	val, err := models.RDB.Get(ctx, redisKey).Result()
	if err == nil && val != "" {
		// 命中 Redis，反序列化并返回
		var res []gin.H
		if err := json.Unmarshal([]byte(val), &res); err == nil {
			c.JSON(http.StatusOK, res)
			return
		}
		// 如果 Redis 反序列化失败，允许继续查数据库（兼容缓存损坏）
	}
	if err != nil && err != redis.Nil {
		// Redis 其它错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis 查询异常"})
		return
	}
	// cache miss，从 MySQL 查
	var items []models.ShopItem
	if err := models.DB.Where("type = ?", "public").Order("created_at desc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法查询商品"})
		return
	}
	res := make([]gin.H, 0, len(items))
	for _, item := range items {
		res = append(res, gin.H{
			"id":         item.ID,
			"name":       item.Name,
			"points":     item.Points,
			"image":      item.Image,
			"created_at": item.CreatedAt.Format("2006-01-02"),
		})
	}
	// 存入 Redis 缓存 24 小时
	if raw, marshalErr := json.Marshal(res); marshalErr == nil {
		models.RDB.Set(ctx, redisKey, raw, 24*time.Hour)
	}
	c.JSON(http.StatusOK, res)
}

// GetPrivateShopItems 查询用户的自定义商品（私有商品）
func GetPrivateShopItems(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	var items []models.ShopItem
	if err := models.DB.Where("user_id = ? AND type = ?", userID, "private").Order("created_at desc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法查询商品"})
		return
	}
	res := make([]gin.H, 0, len(items))
	for _, item := range items {
		res = append(res, gin.H{
			"id":         item.ID,
			"name":       item.Name,
			"points":     item.Points,
			"image":      item.Image,
			"created_at": item.CreatedAt.Format("2006-01-02"),
		})
	}
	c.JSON(http.StatusOK, res)
}

// CreatePrivateShopItem 创建自定义商品（或公共商品）
func CreatePrivateShopItem(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var req struct {
		Name  string  `json:"name"`
		Points float64 `json:"points"` // 前端给的是未折算积分
		Type  string  `json:"type"`
		Image string  `json:"image"`
		Rate  float64 `json:"rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		req.Name == "" || req.Points == 0 || req.Type == "" || req.Image == "" || req.Rate == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数"})
		return
	}

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	points := math.Floor((req.Points/req.Rate)*10) / 10

	item := models.ShopItem{
		Name:   req.Name,
		Points: points,
		Type:   req.Type,
		Image:  req.Image,
		UserID: &userID,
	}
	if err := models.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建商品失败"})
		return
	}
	// 更新用户的 exchange_rate
	models.DB.Model(&user).Update("exchange_rate", req.Rate)

	// 新增：如果创建的是公共商品，清除缓存
	if item.Type == "public" {
		ctx := context.Background()
		models.RDB.Del(ctx, "shop:public_items")
	}

	c.JSON(http.StatusCreated, gin.H{"message": "自定义商品创建成功"})
}

// RedeemItem 兑换商品
func RedeemItem(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	var req struct {
		ItemID uint `json:"item_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少商品id"})
		return
	}

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var item models.ShopItem
	if err := models.DB.First(&item, req.ItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}

	if user.CurrentPoints < item.Points {
		c.JSON(http.StatusBadRequest, gin.H{"error": "积分不足"})
		return
	}

	newPoints := math.Floor((user.CurrentPoints-item.Points)*10) / 10
	models.DB.Model(&user).Update("current_points", newPoints)

	// 如果是私有商品，则删除
	if item.Type == "private" {
		models.DB.Delete(&item)
	}

	// 插入兑换记录
	redeem := models.RedeemedItem{
		UserID:    userID,
		ItemName:  item.Name,
		ItemPoints: item.Points,
		ItemImage: item.Image,
	}
	models.DB.Create(&redeem)

	c.JSON(http.StatusOK, gin.H{
		"message":        "兑换成功",
		"current_points": formatFloatAsString(newPoints),
		"name":           item.Name,
		"type":           item.Type,
	})
}

// ShopInfo 查询商品 type 信息
func ShopInfo(c *gin.Context) {
	// 注意 user_id 必须从 session
	// item_id 从前端 Body 提供
	var req struct {
		ItemID uint `json:"item_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少商品id"})
		return
	}
	var item models.ShopItem
	if err := models.DB.First(&item, req.ItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"type": item.Type})
}

// AlterItem 修改商品积分
func AlterItem(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	var req struct {
		ItemID    uint    `json:"item_id"`
		ItemPoints float64 `json:"item_points"` // 前端给的是折合前的积分（需要重新计算）
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数"})
		return
	}
	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	newPoints := math.Floor((req.ItemPoints/float64(user.ExchangeRate))*10) / 10
	var item models.ShopItem
	if err := models.DB.First(&item, req.ItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	models.DB.Model(&item).Update("points", newPoints)

	// 新增：如果是 public 商品，清除缓存
	if item.Type == "public" {
		ctx := context.Background()
		models.RDB.Del(ctx, "shop:public_items")
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "商品更新成功",
		"name":    item.Name,
	})
}

// DeleteItem 删除商品
func DeleteItem(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	var req struct {
		ItemID uint `json:"item_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少商品id"})
		return
	}
	var item models.ShopItem
	if err := models.DB.First(&item, req.ItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	// 将阻塞的 DB.Count 和 DeleteLocalFile 替换为异步投递
	if strings.TrimSpace(item.Image) != "" {
		select {
		case ImageCleanupChan <- item.Image:
			// 成功将图片 URL 扔进后台清理队列，主线程继续往下走，丝滑无比
		default:
			// 极端情况：队列满了（100个名额满了），为了不阻塞主线程，这里可以选择记录日志或者直接忽略
			// 即使没删掉，也只是一张废图，不影响核心业务逻辑
		}
	}

	models.DB.Delete(&item)

	// 新增：如果是 public 商品，清除缓存
	if item.Type == "public" {
		ctx := context.Background()
		models.RDB.Del(ctx, "shop:public_items")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "商品已成功删除",
		"current_points": formatFloatAsString(user.CurrentPoints),
		"name":           item.Name,
	})
}

// formatFloatAsString 保留最多一位小数，去除末尾为.0的情况
func formatFloatAsString(val float64) string {
	return strings.TrimRight(strings.TrimRight(
		strconv.FormatFloat(val, 'f', 1, 64), "0"), ".")
}

