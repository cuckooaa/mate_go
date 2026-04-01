package models

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"github.com/redis/go-redis/v9"
)

// DB 是全局的 GORM MySQL 连接句柄
var DB *gorm.DB

// RDB 是全局的 Redis 客户端连接句柄
var RDB *redis.Client

// InitDB 初始化 MySQL 和 Redis 的全局连接
func InitDB() {

	// 构建 MySQL 连接字符串
	// DSN 格式: 用户名:密码@tcp(主机:端口)/数据库?参数
	dsn := "root:rootpassword@tcp(127.0.0.1:3406)/mate_db?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"

	// 连接 MySQL，使用 GORM
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		// 连接失败直接 panic
		panic(fmt.Sprintf("无法连接到 MySQL 数据库: %v", err))
	}

	// 设置 MySQL 连接池参数（可选）
	sqlDB, err := DB.DB()
	if err != nil {
		panic(fmt.Sprintf("获取 MySQL 连接池出错: %v", err))
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	// 初始化 Redis 客户端
	RDB = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // 本地 Redis
		Password: "",               // 无密码
		DB:       0,                // 默认数据库
	})

	// 测试 Redis 是否连接成功
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = RDB.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("无法连接到 Redis: %v", err))
	}
}
