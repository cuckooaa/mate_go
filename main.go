package main

import (
	"mate/models"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/redis"
    "mate/controllers"
	"mate/middlewares"
	"os"
)

func main() {
	// 初始化数据库
	models.InitDB()

	// 自动迁移 User 表结构
	models.DB.AutoMigrate(
		&models.User{},
		&models.Task{},
		&models.WeekRecord{},
		&models.ShopItem{},
		&models.RedeemedItem{},
	)

	// 启动后台文件异步清理服务
	controllers.StartImageCleanupWorker()

	// 初始化 Gin 默认引擎
	r := gin.Default()

	// 核心：告诉 Gin 加载 templates 文件夹下的所有 HTML 文件
	r.LoadHTMLGlob("templates/*")

	// 配置 Redis Session
	// securecookie uses hashKey/blockKey to sign (and optionally encrypt) the session cookie.
	// If not provided, session.Save() will fail with: "securecookie: no codecs provided".
	hashKey := os.Getenv("MATE_SESSION_HASH_KEY")
	blockKey := os.Getenv("MATE_SESSION_BLOCK_KEY")
	if hashKey == "" {
		// 32 bytes default for local development only.
		hashKey = "0123456789abcdef0123456789abcdef"
	}
	if blockKey == "" {
		// 32 bytes = AES-256 block key. Optional encryption; using it makes cookies confidential.
		blockKey = "fedcba9876543210fedcba9876543210"
	}

	store, err := redis.NewStore(
		10,
		"tcp",
		"127.0.0.1:6379",
		"",
		"",
		[]byte(hashKey),
		[]byte(blockKey),
	)
	if err != nil {
		panic(err)
	}
	r.Use(sessions.Sessions("mate_session", store))

	// 公开 API 路由 (不需要登录)
	publicAPI := r.Group("/api")
	{
		publicAPI.POST("/register", controllers.Register)
		publicAPI.POST("/login", controllers.Login)
		publicAPI.GET("/check_login_status", controllers.CheckLoginStatus) // 供前端检查状态
		publicAPI.POST("/logout", controllers.Logout)
		publicAPI.GET("/leaderboard", controllers.GetLeaderboard)
		// Debug: 帮助定位 cookie / session 是否正常
		// publicAPI.GET("/debug_session", func(c *gin.Context) {
		// 	sess := sessions.Default(c)
		// 	mateSession, _ := c.Cookie("mate_session")
		// 	c.JSON(200, gin.H{
		// 		"cookie_header":  c.GetHeader("Cookie"),
		// 		"mate_session":   mateSession,
		// 		"session_id":     sess.ID(),
		// 		"user_id":        sess.Get("user_id"),
		// 	})
		// })
	}

	// 保护 API 路由 (需要登录鉴权)
	protectedAPI := r.Group("/api")
	protectedAPI.Use(middlewares.AuthMiddleware()) // 核心：使用你刚写的保安中间件
	{
		// 测试一下保护路由
		protectedAPI.GET("/me", func(c *gin.Context) {
			userID := c.MustGet("user_id").(string)
			c.JSON(200, gin.H{"message": "成功访问受保护的数据", "your_user_id": userID})
		})
		// 之后的所有任务、商城 API 都会写在这里！

		// --- 计划页面 API (日常任务) ---
		protectedAPI.POST("/tasks", controllers.CreateTask)                // 创建任务
		protectedAPI.GET("/tasks", controllers.GetTasks)                   // 获取任务列表 (注意: 移除了 URL 里的 /:user_id)
		protectedAPI.POST("/tasks/:id/complete", controllers.CompleteTask) // 完成任务
		protectedAPI.DELETE("/tasks/:id", controllers.DeleteTask)          // 删除任务 (注意: 移除了 /delete 后缀，遵循 RESTful 风格)
		protectedAPI.POST("/tasks/:id/track", controllers.TrackTaskTime)   // 同步计时

		// --- 每周结算 API ---
		// 注意：我们在路由里保留了 /:user_id 仅仅是为了兼容前端 fetch 请求的 URL 格式
		// 但在控制器内部，我们绝对不使用它，而是通过 session 保证安全
		protectedAPI.GET("/:user_id/lastSettlementDate", controllers.GetLastSettlementDate) 
		protectedAPI.POST("/tasks/settlement", controllers.SettleTasks)
		protectedAPI.GET("/weekrecord/:id", controllers.GetWeekRecordDetails) // 前端获取每周详细任务的接口

		// --- 商城页面 API ---
		protectedAPI.GET("/shop/public", controllers.GetPublicShopItems)
		// 注意：去掉了 URL 里的 /:user_id，前端如果还是请求原 URL，可以在前端稍作修改，或者在 Go 里写 "/shop/private/:user_id" 但内部忽略参数
		protectedAPI.GET("/shop/private/:user_id", controllers.GetPrivateShopItems) 
		protectedAPI.POST("/shop/private", controllers.CreatePrivateShopItem)
		protectedAPI.POST("/shop/redeem", controllers.RedeemItem)
		protectedAPI.POST("/shop/info", controllers.ShopInfo)
		protectedAPI.POST("/shop/alter", controllers.AlterItem)
		protectedAPI.DELETE("/shop/delete", controllers.DeleteItem)

		// --- 我的页面 API (需要登录) ---
		// URL 里的 /:user_id 仅做兼容，内部走 Session
		protectedAPI.GET("/user/:user_id", controllers.GetUserInfo) 
		protectedAPI.POST("/user/nickname", controllers.UpdateNickname)
		protectedAPI.POST("/user/update_avatar", controllers.UpdateAvatar)

		// --- 文件上传 API ---
		protectedAPI.POST("/upload", controllers.UploadFile)
	}

	// 配置静态文件服务
	r.Static("/static", "./static")
	r.Static("/uploads", "./uploads")

	// 1. 主页
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// 2. 排行榜页面
	r.GET("/rankinglist", func(c *gin.Context) {
		c.HTML(200, "rankinglist.html", nil)
	})

	// 3. 每周结算详情页面
	r.GET("/weekrecord", func(c *gin.Context) {
		// 从 URL 参数中提取 week_record 的值（如 /weekrecord?week_record=1）
		weekRecordID := c.Query("week_record")
		// 将其通过模板变量注入到 HTML 中
		c.HTML(200, "weekrecord.html", gin.H{
			"week_record": weekRecordID,
		})
	})

	// 注册测试路由
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// 启动服务，监听 8080 端口
	r.Run(":5000")
}