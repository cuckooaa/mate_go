package controllers

import (
	"time"
	"math/rand"
	"strconv"
	"strings"
	"mate/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"github.com/gin-contrib/sessions"
)

// RegisterInput 用于注册请求的绑定
type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Nickname string `json:"nickname"`
}

// LoginInput 用于登录请求的绑定
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// 生成伪 uuid（如未引入 uuid 库，可以用这种方式）
func genUID() string {
	t := time.Now().UnixNano()
	r := rand.Intn(1000000)
	return strconv.FormatInt(t, 10) + "_" + strconv.Itoa(r)
}

// Register 用户注册接口
func Register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	input.Email = strings.TrimSpace(input.Email)
	if input.Nickname == "" {
		input.Nickname = "新用户"
	}

	// 检查邮箱是否已存在
	var count int64
	models.DB.Model(&models.User{}).Where("email = ?", input.Email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已被注册"})
		return
	}

	// 密码加密
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	user := models.User{
		ID:                genUID(),
		Email:             input.Email,
		Password:          string(hash),
		Nickname:          input.Nickname,
		Avatar:            "/static/a2.png",
		CurrentPoints:     0,
		TotalEarnedPoints: 0,
		LastSettlementDate: time.Now(),
		ExchangeRate:      10,
	}

	if err := models.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户创建失败"})
		return
	}

	// 注册成功自动设置 session
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存会话失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"user_id": user.ID,
		"user": gin.H{
			"id":                user.ID,
			"email":             user.Email,
			"nickname":          user.Nickname,
			"current_points":    user.CurrentPoints,
			"total_earned_points": user.TotalEarnedPoints,
			"avatar":            user.Avatar,
			"last_settlement_date": user.LastSettlementDate,
		},
	})
}

// Login 用户登录接口
func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	var user models.User
	err := models.DB.Where("email = ?", strings.TrimSpace(input.Email)).First(&user).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 校验密码
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 登录成功，写入 session
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存会话失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"user_id": user.ID,
		"user": gin.H{
			"id":                user.ID,
			"email":             user.Email,
			"nickname":          user.Nickname,
			"current_points":    user.CurrentPoints,
			"total_earned_points": user.TotalEarnedPoints,
			"avatar":            user.Avatar,
			"last_settlement_date": user.LastSettlementDate,
		},
	})
}

// CheckLoginStatus 用户登录状态检查接口
func CheckLoginStatus(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID != nil {
		c.JSON(http.StatusOK, gin.H{
			"is_logged_in": true,
			"user_id":      userID,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"is_logged_in": false,
	})
}

// Logout 用户登出接口
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存会话失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}
