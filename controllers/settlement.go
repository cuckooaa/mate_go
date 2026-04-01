package controllers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"mate/models"
)

// SettleTasks handles weekly settlement of completed tasks for a user.
func SettleTasks(c *gin.Context) {
	// 强制使用认证中间件设置的 user_id
	userID := c.MustGet("user_id").(string)

	var req struct {
		SeasonWeek string `json:"season_week"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var tasks []models.Task
	if err := models.DB.Where("user_id = ? AND status = ?", userID, "completed").Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "任务查询失败"})
		return
	}

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var totalPoints float64
	var totalTime int
	var taskIDs []uint

	if len(tasks) == 0 {
		// 没有需要结算的任务时，也要更新时间
		user.LastSettlementDate = time.Now().UTC()
		models.DB.Save(&user)
		c.JSON(http.StatusOK, gin.H{"message": "没有已完成的任务需要结算"})
		return
	}

	for _, task := range tasks {
		totalPoints += task.Points
		totalTime += task.TimeSpentSeconds
		taskIDs = append(taskIDs, task.ID)

		// 归档已完成任务
		task.Status = "archived"
		models.DB.Model(&models.Task{}).Where("id = ?", task.ID).Update("status", "archived")
	}
	// 保留 1 位小数
	totalPoints = math.Floor(totalPoints*10) / 10

	taskCount := len(tasks)

	// TaskLists 需要序列化为 JSON 字符串
	taskListBytes, _ := json.Marshal(taskIDs)

	newRecord := models.WeekRecord{
		UserID:     userID,
		Name:       req.SeasonWeek,
		TotalPoints: totalPoints,
		TotalNum:   taskCount,
		TaskLists:  string(taskListBytes),
		TotalTime:  totalTime,
	}
	if err := models.DB.Create(&newRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "结算记录创建失败"})
		return
	}

	user.LastSettlementDate = time.Now().UTC()
	models.DB.Save(&user)

	aveDayTime := int(math.Floor(float64(totalTime) / 7))
	totalHours := totalTime / 3600
	totalMin := (totalTime % 3600) / 60
	totalSec := totalTime % 60
	aveHours := aveDayTime / 3600
	aveMin := (aveDayTime % 3600) / 60
	aveSec := aveDayTime % 60

	// 响应内容和 Python 逻辑一致
	message := "😻本周共完成 " + strconv.Itoa(taskCount) + " 项任务，获得 " +
		strconv.FormatFloat(totalPoints, 'f', 1, 64) +
		" 积分。总共花费 " + strconv.Itoa(totalHours) + "小时" + strconv.Itoa(totalMin) + "分钟" +
		strconv.Itoa(totalSec) + "秒。每天平均花费" + strconv.Itoa(aveHours) + "小时" + strconv.Itoa(aveMin) +
		"分钟" + strconv.Itoa(aveSec) + "秒。已完成的任务已被归档，让我们迎接新的一周，制定新的目标和计划吧！"

	c.JSON(http.StatusOK, gin.H{"message": message})
}

// GetLastSettlementDate 获取用户周结算时间
func GetLastSettlementDate(c *gin.Context) {
	userID := c.MustGet("user_id").(string)

	var user models.User
	if err := models.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var last string
	if !user.LastSettlementDate.IsZero() {
		last = user.LastSettlementDate.UTC().Format(time.RFC3339)
	} else {
		last = ""
	}
	c.JSON(http.StatusOK, gin.H{"last_settlement_date": last + "Z"})
}

// GetWeekRecordDetails 查询某周的任务详情
func GetWeekRecordDetails(c *gin.Context) {
	weekrecordIDStr := c.Param("id")
	weekrecordID, err := strconv.Atoi(weekrecordIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var wr models.WeekRecord
	if err := models.DB.Where("id = ?", weekrecordID).First(&wr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "周结算记录不存在"})
		return
	}

	// 反序列化 TaskLists
	var taskIDs []uint
	_ = json.Unmarshal([]byte(wr.TaskLists), &taskIDs)

	var tasks []models.Task
	if len(taskIDs) > 0 {
		if err := models.DB.Where("id IN ?", taskIDs).Find(&tasks).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "任务查询失败"})
			return
		}
	}

	const shanghaiZone = "Asia/Shanghai"
	loc, _ := time.LoadLocation(shanghaiZone)
	type TaskResp struct {
		ID               uint    `json:"id"`
		Description      string  `json:"description"`
		Type             string  `json:"type"`
		Points           float64 `json:"points"`
		Status           string  `json:"status"`
		CreatedAt        string  `json:"created_at"`
		TimeSpentSeconds int     `json:"time_spent_seconds"`
		TimerStartTime   *float64 `json:"timer_start_time,omitempty"`
	}
	taskList := make([]TaskResp, 0, len(tasks))
	for _, task := range tasks {
		tCreated := task.CreatedAt.In(loc).Format("2006-01-02")
		taskList = append(taskList, TaskResp{
			ID:               task.ID,
			Description:      task.Description,
			Type:             task.Type,
			Points:           task.Points,
			Status:           task.Status,
			CreatedAt:        tCreated,
			TimeSpentSeconds: task.TimeSpentSeconds,
			TimerStartTime:   task.TimerStartTime,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"tasks": taskList,
		"name":  wr.Name,
	})
}
