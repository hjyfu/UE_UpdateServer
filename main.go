package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"hotupdate/app/controllers"
	"hotupdate/app/models"
)

// Config 服务器配置
type Config struct {
	Server struct {
		Port int    `json:"port"`
		Host string `json:"host"`
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
	port       = flag.String("port", "9090", "服务器端口")
	uploadDir  = flag.String("upload", "./uploads", "上传文件存放目录")
	logDir     = flag.String("log", "./logs", "日志文件存放目录")
	configFile = flag.String("config", "./config.json", "配置文件路径")
	config     Config
)

func main() {
	flag.Parse()

	// 加载配置文件
	if err := loadConfig(); err != nil {
		log.Printf("警告: 无法加载配置文件: %v, 将使用命令行参数", err)
	}

	// 确保目录存在
	ensureDir(*uploadDir)
	ensureDir(*logDir)

	// 初始化日志
	initLogger()

	// 初始化版本管理
	initVersions()

	// 设置Gin路由
	r := setupRouter()

	// 获取实际要使用的端口
	portToUse := *port
	if config.Server.Port > 0 {
		portToUse = strconv.Itoa(config.Server.Port)
	}

	// 启动服务器
	log.Printf("多项目热更新服务器已启动，监听端口 %s\n", portToUse)
	log.Fatal(r.Run(":" + portToUse))
}

// 加载配置文件
func loadConfig() error {
	// 检查配置文件是否存在
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", *configFile)
	}

	// 读取配置文件
	data, err := os.ReadFile(*configFile)
	if err != nil {
		return err
	}

	// 解析JSON
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	// 如果配置文件中设置了这些值，则覆盖命令行参数
	if config.Storage.UploadDir != "" {
		*uploadDir = config.Storage.UploadDir
	}
	if config.Storage.LogDir != "" {
		*logDir = config.Storage.LogDir
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
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			log.Fatalf("无法创建目录 %s: %v", dir, err)
		}
		log.Printf("已创建目录: %s", dir)
	}
}

// 初始化日志系统
func initLogger() {
	logFile := filepath.Join(*logDir, "server.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}
	log.SetOutput(f)
	log.SetPrefix("[热更新服务] ")
	log.Println("日志系统已初始化")
}

// 初始化版本管理
func initVersions() {
	versionsDir := filepath.Join(*uploadDir, "versions")
	ensureDir(versionsDir)

	// 检查是否需要创建初始版本
	versionJsonPath := filepath.Join(*uploadDir, "versions.json")
	if _, err := os.Stat(versionJsonPath); os.IsNotExist(err) {
		// 创建初始版本信息
		createInitialVersion()
	}
}

// 创建初始版本
func createInitialVersion() {
	log.Println("创建初始版本信息")

	// 创建初始版本
	versionList, err := models.CreateInitialVersion(*uploadDir, "")
	if err != nil {
		log.Fatalf("无法创建初始版本: %v", err)
	}

	// 打印版本信息
	log.Printf("初始版本已创建: %s", versionList.LatestVersion)
}

// 设置Gin路由
func setupRouter() *gin.Engine {
	r := gin.Default()

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
	controllers.SetupVersionController(r, *uploadDir)

	return r
}
