package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hotupdate/app/controllers"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Config 服务器配置
type Config struct {
	Server struct {
		Port      int    `json:"port"`
		Host      string `json:"host"`
		DebugMode bool   `json:"debugMode"`
	} `json:"server"`
	Storage struct {
		UploadDir string `json:"uploadDir"`
		LogDir    string `json:"logDir"`
	} `json:"storage"`
	Version struct {
		InitialVersion            string `json:"initialVersion"`
		InitialVersionName        string `json:"initialVersionName"`
		InitialVersionDescription string `json:"initialVersionDescription"`
	} `json:"version"`
	Security struct {
		AdminUsername string `json:"adminUsername"`
		AdminPassword string `json:"adminPassword"`
	} `json:"security"`
	Apps []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"apps"`
}

var (
	port       int
	uploadDir  string
	logDir     string
	configPath string
	debug      bool
	config     Config
)

func main() {
	initConfig()

	// 初始化日志
	initLogger()

	log.Println("正在启动多项目热更新服务器...")
	startTime := time.Now()

	// 确保目录存在
	ensureDir(uploadDir)
	ensureDir(logDir)

	// 设置Gin模式
	if debug || config.Server.DebugMode {
		gin.SetMode(gin.DebugMode)
		log.Println("以调试模式运行")
	} else {
		gin.SetMode(gin.ReleaseMode)
		log.Println("以生产模式运行")
	}

	// 设置Gin路由
	r := setupRouter()

	// 获取实际要使用的端口
	portToUse := strconv.Itoa(port)

	// 启动前准备所需时间
	log.Printf("服务器准备完成，耗时 %v", time.Since(startTime))

	// 启动提示
	hostAddr := fmt.Sprintf(":%s", portToUse)
	log.Printf("多项目热更新服务器已启动，监听 %s", hostAddr)
	log.Printf("管理界面: http://localhost:%s/admin", portToUse)
	log.Printf("健康检查: http://localhost:%s/health", portToUse)

	// 启动服务器
	if err := r.Run(hostAddr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

// 初始化配置
func initConfig() {
	// 读取命令行参数
	flag.StringVar(&configPath, "config", "./config.json", "配置文件路径")
	flag.IntVar(&port, "port", 9090, "服务器端口")
	flag.StringVar(&uploadDir, "upload", "./uploads", "上传目录")
	flag.StringVar(&logDir, "log", "./logs", "日志目录")
	flag.BoolVar(&debug, "debug", false, "调试模式")
	flag.Parse()

	// 尝试从环境变量读取配置，环境变量优先级高于命令行参数
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	if envUploadDir := os.Getenv("UPLOAD_DIR"); envUploadDir != "" {
		uploadDir = envUploadDir
	}

	if envLogDir := os.Getenv("LOG_DIR"); envLogDir != "" {
		logDir = envLogDir
	}

	if envConfigPath := os.Getenv("CONFIG_PATH"); envConfigPath != "" {
		configPath = envConfigPath
	}

	if envDebug := os.Getenv("DEBUG_MODE"); envDebug != "" {
		debug = (envDebug == "true" || envDebug == "1")
	}

	// 加载配置文件
	if err := loadConfig(); err != nil {
		log.Printf("无法加载配置文件: %v, 使用默认配置", err)
	}

	// 环境变量可以覆盖配置文件中的设置
	if envHost := os.Getenv("HOST"); envHost != "" {
		config.Server.Host = envHost
	}

	if envDebugMode := os.Getenv("DEBUG_MODE"); envDebugMode != "" {
		config.Server.DebugMode = (envDebugMode == "true" || envDebugMode == "1")
	}
}

// 加载配置文件
func loadConfig() error {
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// 解析JSON
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	log.Printf("成功加载配置文件: %s", configPath)

	// 如果配置文件中设置了这些值，则覆盖命令行参数
	if config.Storage.UploadDir != "" {
		uploadDir = config.Storage.UploadDir
		log.Printf("使用配置文件中的上传目录: %s", uploadDir)
	}
	if config.Storage.LogDir != "" {
		logDir = config.Storage.LogDir
		log.Printf("使用配置文件中的日志目录: %s", logDir)
	}

	// 预先创建配置文件中指定的应用
	if len(config.Apps) > 0 {
		log.Printf("从配置文件中加载 %d 个预定义应用", len(config.Apps))
		// 稍后在初始化应用列表时处理
	}

	return nil
}

// 确保目录存在
func ensureDir(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		log.Println("警告: 目录路径为空")
		return
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			log.Fatalf("无法创建目录 %s: %v", dir, err)
		}
		log.Printf("已创建目录: %s", dir)
	} else {
		log.Printf("目录已存在: %s", dir)
	}
}

// 初始化日志系统
func initLogger() {
	// 确保日志目录存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// 如果无法创建日志目录，继续使用标准输出
			log.Printf("无法创建日志目录: %v, 将使用标准输出", err)
			return
		}
	}

	// 生成带时间戳的日志文件名
	timestamp := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logDir, fmt.Sprintf("server_%s.log", timestamp))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("无法打开日志文件: %v, 将使用标准输出", err)
		return
	}

	// 同时输出到文件和控制台
	log.SetOutput(io.MultiWriter(f, os.Stdout))
	log.SetPrefix("[热更新服务] ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("日志系统已初始化，日志文件:", logFile)
}

// 设置Gin路由
func setupRouter() *gin.Engine {
	r := gin.Default()

	// 设置Gin恢复中间件
	r.Use(gin.Recovery())

	// 添加请求日志中间件
	r.Use(func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 结束时间
		endTime := time.Now()

		// 请求处理时间
		latencyTime := endTime.Sub(startTime)

		// 请求方式
		reqMethod := c.Request.Method

		// 请求路由
		reqUri := c.Request.RequestURI

		// 状态码
		statusCode := c.Writer.Status()

		// 请求IP
		clientIP := c.ClientIP()

		// 日志格式
		log.Printf("| %3d | %13v | %15s | %-7s | %s",
			statusCode,
			latencyTime,
			clientIP,
			reqMethod,
			reqUri,
		)
	})

	// 静态文件
	r.Static("/static", "./app/static")
	r.LoadHTMLGlob("app/views/templates/*")

	// 管理界面
	r.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "admin.html", gin.H{
			"title": "多项目热更新管理系统",
		})
	})

	// 首页重定向到管理界面
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/admin")
	})

	// 设置版本控制器
	controllers.SetupVersionController(r, uploadDir)

	return r
}
