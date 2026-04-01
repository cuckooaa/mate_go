package models

import (
	"time"
)

type User struct {
	ID                string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Email             string    `gorm:"type:varchar(120);uniqueIndex;not null" json:"email"`
	Password          string    `gorm:"type:varchar(120);not null" json:"password"`
	CurrentPoints     float64   `gorm:"default:0" json:"current_points"`
	TotalEarnedPoints float64   `gorm:"default:0" json:"total_earned_points"`
	Nickname          string    `gorm:"type:varchar(80)" json:"nickname"`
	Avatar            string    `gorm:"type:varchar(255)" json:"avatar"`
	LastSettlementDate time.Time `gorm:"type:datetime;default:CURRENT_TIMESTAMP" json:"last_settlement_date"`
	ExchangeRate      int       `gorm:"default:10" json:"exchange_rate"`
}