package controllers

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"mate/models"
)

// CreateTask 创建任务
func CreateTask(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var req struct {
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Description == "" || req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要的任务信息"})
		return
	}

	pointsMap := map[string]float64{
		"s-u-c":    5,
		"ns-u-c":   3,
		"s-nu-c":   4,
		"s-u-nc":   4,
		"ns-nu-c":  3,
		"ns-u-nc":  2,
		"s-nu-nc":  3,
		"ns-nu-nc": 1,
	}
	points, ok := pointsMap[req.Type]
	if !ok {
		points = 0
	}

	task := &models.Task{
		UserID:           userID,
		Description:      req.Description,
		Type:             req.Type,
		Points:           points,
		Status:           "pending",
		TimeSpentSeconds: 0,
		CreatedAt:        time.Now(),
	}
	if err := models.DB.Create(task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "任务创建失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "任务创建成功",
		"task": gin.H{
			"id":          task.ID,
			"description": task.Description,
			"type":        task.Type,
			"points":      task.Points,
			"status":      task.Status,
		},
	})
}

// GetTasks 获取任务列表和周记录
func GetTasks(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var tasks []models.Task
	if err := models.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询任务失败"})
		return
	}

	// WeekRecord（按时间降序）
	var weekRecords []models.WeekRecord
	if err := models.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&weekRecords).Error; err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询周记录失败"})
		return
	}

	var weekRecordList []gin.H
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, record := range weekRecords {
		createdAt := record.CreatedAt.In(loc).Format("2006-01-02")
		weekRecordList = append(weekRecordList, gin.H{
			"id":           record.ID,
			"name":         record.Name,
			"total_points": record.TotalPoints,
			"total_num":    record.TotalNum,
			"created_at":   createdAt,
			"total_time":   record.TotalTime,
		})
	}

	var tasksList []gin.H
	for _, t := range tasks {
		createdAt := t.CreatedAt.In(loc).Format("2006-01-02")
		tasksList = append(tasksList, gin.H{
			"id":                 t.ID,
			"description":        t.Description,
			"type":               t.Type,
			"points":             t.Points,
			"status":             t.Status,
			"created_at":         createdAt,
			"time_spent_seconds": t.TimeSpentSeconds,
			"timer_start_time":   t.TimerStartTime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks_list":  tasksList,
		"week_record": weekRecordList,
	})
}

// CompleteTask 完成任务
func CompleteTask(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	taskIDstr := c.Param("id")

	taskID, err := strconv.Atoi(taskIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var task models.Task
	if err := models.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或不属于该用户"})
		return
	}
	if task.TimeSpentSeconds < 600 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请确保任务计时达到10分钟以上，再点击完成"})
		return
	}

	// 按Python的公式计算积分
	points := (task.Points*0.5 + 1.5) * 0.005 * (float64(task.TimeSpentSeconds) / 60.0)
	points = math.Floor(points*10) / 10

	task.Status = "completed"
	task.Points = points

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err == nil {
		user.CurrentPoints += points
		user.TotalEarnedPoints += points
		user.CurrentPoints = math.Floor(user.CurrentPoints*10) / 10
		user.TotalEarnedPoints = math.Floor(user.TotalEarnedPoints*10) / 10
		models.DB.Save(&user)
	}

	if err := models.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "任务完成保存失败"})
		return
	}

	ctx := context.Background()
	_ = models.RDB.ZRem(ctx, leaderboardCacheKey, leaderboardEmptyMember).Err()
	if _, err := models.RDB.ZIncrBy(ctx, leaderboardCacheKey, points, userID).Result(); err == nil {
		models.RDB.Expire(ctx, leaderboardCacheKey, 24*time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "任务已完成",
		"points":             points,
		"time_spent_seconds": task.TimeSpentSeconds,
	})
}

// DeleteTask 删除任务
func DeleteTask(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	taskIDstr := c.Param("id")
	taskID, err := strconv.Atoi(taskIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var task models.Task
	if err := models.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或不属于该用户"})
		return
	}
	if err := models.DB.Delete(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "任务删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已成功删除"})
}

// TrackTaskTime 任务计时数据同步
func TrackTaskTime(c *gin.Context) {
	userID := c.MustGet("user_id").(string)
	taskIDstr := c.Param("id")
	taskID, err := strconv.Atoi(taskIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var req struct {
		TimeSpent      *int     `json:"time_spent"`       // 新增的计时秒数
		TimerStartTime *float64 `json:"timer_start_time"` // 计时器开始时间，秒时间戳或nil
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var task models.Task
	if err := models.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或不属于该用户"})
		return
	}
	// 1. 累计总时间
	if req.TimeSpent != nil {
		task.TimeSpentSeconds += *req.TimeSpent
	}
	// 2. 更新计时器开始时间
	if req.TimerStartTime != nil {
		task.TimerStartTime = req.TimerStartTime
	} else {
		task.TimerStartTime = nil
	}
	if err := models.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步计时数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "计时数据已同步",
		"time_spent_seconds": task.TimeSpentSeconds,
		"timer_start_time":   task.TimerStartTime,
	})
}
