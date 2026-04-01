package models

import (
	"time"
)

type Task struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID          string    `json:"user_id" gorm:"type:varchar(255);index"`
	Description     string    `json:"description" gorm:"type:varchar(255);not null"`
	Type            string    `json:"type" gorm:"type:varchar(20);not null"`
	Points          float64   `json:"points" gorm:"not null"`
	Status          string    `json:"status" gorm:"type:varchar(20);default:'pending'"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	TimeSpentSeconds int      `json:"time_spent_seconds" gorm:"default:0"`
	TimerStartTime  *float64  `json:"timer_start_time,omitempty" gorm:"type:float"`
}

type WeekRecord struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     string    `json:"user_id" gorm:"type:varchar(255);index"`
	Name       string    `json:"name" gorm:"type:varchar(100);not null"`
	TotalNum   int       `json:"total_num" gorm:"not null"`
	TotalPoints float64  `json:"total_points" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	TotalTime  int       `json:"total_time" gorm:"default:0"`
	TaskLists  string    `json:"task_lists" gorm:"type:text"` // 可使用自定义类型或 JSON 序列化字符串
}

type ShopItem struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null"`
	Points    float64   `json:"points" gorm:"not null"`
	Image     string    `json:"image" gorm:"type:varchar(255)"`
	Type      string    `json:"type" gorm:"type:varchar(20);not null"`
	UserID    *string   `json:"user_id,omitempty" gorm:"type:varchar(255);index"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type RedeemedItem struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    string    `json:"user_id" gorm:"type:varchar(255);index"`
	ItemName  string    `json:"item_name" gorm:"type:varchar(100);not null"`
	ItemPoints float64  `json:"item_points" gorm:"not null"`
	RedeemedAt time.Time `json:"redeemed_at" gorm:"autoCreateTime"`
	ItemImage string    `json:"item_image" gorm:"type:varchar(255)"`
}
