package controllers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hotupdate/app/models"
)

var (
	UploadDir    string
	AppsJsonPath string
	isReady      = false // 服务就绪标志
)

// SetupVersionController 设置版本控制器
func SetupVersionController(r *gin.Engine, uploadDirectory string) {
	UploadDir = uploadDirectory
	AppsJsonPath = filepath.Join(UploadDir, "apps.json")

	// 健康检查API
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"ready":   isReady,
			"version": "1.0.0",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// 应用管理API
	r.POST("/api/apps", CreateApp)
	r.GET("/api/apps", ListApps)
	r.GET("/api/apps/:app_id", GetAppInfo)
	r.DELETE("/api/apps/:app_id", DeleteApp)

	// 版本管理API
	r.POST("/api/apps/:app_id/versions", CreateVersion)
	r.GET("/api/apps/:app_id/versions", ListVersions)

	// 客户端API
	r.GET("/api/apps/:app_id/check", CheckUpdate)
	r.GET("/api/apps/:app_id/download/:version/:filename", DownloadFile)

	// 为了保持向后兼容，保留原有API（不带app_id的路径），但内部会使用"default"应用
	r.POST("/api/versions", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "app_id", Value: "default"})
		CreateVersion(c)
	})
	r.GET("/api/versions", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "app_id", Value: "default"})
		ListVersions(c)
	})
	r.GET("/api/check", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "app_id", Value: "default"})
		CheckUpdate(c)
	})
	r.GET("/api/download/:version/:filename", func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "app_id", Value: "default"})
		DownloadFile(c)
	})

	// 初始化应用列表，确保至少有一个默认应用
	go func() {
		initApps()
		isReady = true
		log.Println("热更新服务器初始化完成，所有API已就绪")
	}()
}

// 初始化应用列表，确保至少有一个默认应用
func initApps() {
	// 确保apps.json存在
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		log.Printf("加载应用列表失败: %v", err)
		appList = &models.AppList{Apps: []models.App{}}
	}

	// 检查是否有默认应用
	_, hasDefaultApp := models.GetApp(appList, "default")
	if !hasDefaultApp {
		// 创建默认应用
		defaultApp := models.App{
			ID:          "default",
			Name:        "默认应用",
			Description: "系统默认应用",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		appList = models.AddApp(appList, defaultApp)

		// 保存应用列表
		if err := models.SaveApps(appList, AppsJsonPath); err != nil {
			log.Printf("保存应用列表失败: %v", err)
		}

		// 创建应用目录
		if err := models.CreateAppDirectories(UploadDir, "default"); err != nil {
			log.Printf("创建默认应用目录失败: %v", err)
		}

		// 创建初始版本
		createInitialVersionForApp("default")
	}

	// 初始化完成后标记服务就绪
	log.Println("应用初始化完成")
}

// CreateApp 创建新应用
func CreateApp(c *gin.Context) {
	// 改为解析multipart表单
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法解析表单"})
		return
	}

	// 获取表单数据
	appID := c.PostForm("id")
	name := c.PostForm("name")
	description := c.PostForm("description")

	// 验证应用ID
	if appID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID不能为空"})
		return
	}

	// 不允许使用保留的ID
	if appID == "apps" || appID == "api" || appID == "admin" || appID == "static" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID不能使用保留字"})
		return
	}

	// 检查是否有初始版本文件上传
	file, header, err := c.Request.FormFile("initial_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要上传初始版本文件"})
		return
	}
	defer file.Close()

	// 检查文件类型
	if !strings.HasSuffix(header.Filename, ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只接受ZIP文件"})
		return
	}

	// 设置创建时间和更新时间
	now := time.Now()
	app := models.App{
		ID:          appID,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 加载应用列表
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	// 检查应用ID是否已存在
	if _, exists := models.GetApp(appList, app.ID); exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "应用ID已存在"})
		return
	}

	// 添加新应用
	appList = models.AddApp(appList, app)

	// 保存应用列表
	if err := models.SaveApps(appList, AppsJsonPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存应用列表失败"})
		return
	}

	// 创建应用目录
	if err := models.CreateAppDirectories(UploadDir, app.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建应用目录失败"})
		return
	}

	// 创建初始版本目录
	versionId := "1.0.0"
	appDir := models.GetAppUploadDir(UploadDir, app.ID)
	versionDir := filepath.Join(appDir, "versions", versionId)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建版本目录"})
		return
	}

	// 保存上传的文件
	zipPath := filepath.Join(versionDir, "update.zip")
	out, err := os.Create(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建初始版本文件"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存初始版本文件"})
		return
	}

	// 获取文件信息
	fileInfo, err := os.Stat(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取文件信息"})
		return
	}

	// 创建初始版本信息
	initialVersion := models.Version{
		ID:          versionId,
		Name:        "初始版本",
		Description: "系统初始版本",
		FilePath:    filepath.Join("versions", versionId, "update.zip"),
		FileSize:    fileInfo.Size(),
		CreatedAt:   now,
		Force:       false,
	}

	versionList := &models.VersionList{
		Versions:      []models.Version{initialVersion},
		LatestVersion: versionId,
	}

	// 保存版本信息
	versionJsonPath := models.GetAppVersionsJsonPath(UploadDir, app.ID)
	if err := models.SaveVersions(versionList, versionJsonPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存版本信息失败"})
		return
	}

	log.Printf("已创建新应用: %s，并上传初始版本文件", app.ID)
	c.JSON(http.StatusOK, gin.H{
		"message":        "应用创建成功",
		"app":            app,
		"initialVersion": initialVersion,
	})
}

// ListApps 列出所有应用
func ListApps(c *gin.Context) {
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	c.JSON(http.StatusOK, appList)
}

// GetAppInfo 获取应用信息
func GetAppInfo(c *gin.Context) {
	appID := c.Param("app_id")

	// 加载应用列表
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	// 查找应用
	app, exists := models.GetApp(appList, appID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 加载版本信息
	versionJsonPath := models.GetAppVersionsJsonPath(UploadDir, appID)
	versionList, err := models.LoadVersions(versionJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载版本列表"})
		return
	}

	// 返回应用和版本信息
	c.JSON(http.StatusOK, gin.H{
		"app":      app,
		"versions": versionList,
	})
}

// DeleteApp 删除应用
func DeleteApp(c *gin.Context) {
	appID := c.Param("app_id")

	// 不允许删除默认应用
	if appID == "default" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除默认应用"})
		return
	}

	// 加载应用列表
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	// 检查应用是否存在
	if _, exists := models.GetApp(appList, appID); !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 删除应用
	appList = models.DeleteApp(appList, appID)

	// 保存应用列表
	if err := models.SaveApps(appList, AppsJsonPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存应用列表失败"})
		return
	}

	// 删除应用目录（可选，取决于是否要保留历史数据）
	// 这里我们不实际删除文件，只是返回成功

	log.Printf("已删除应用: %s", appID)
	c.JSON(http.StatusOK, gin.H{"message": "应用删除成功"})
}

// 为应用创建初始版本
func createInitialVersionForApp(appID string) {
	log.Printf("为应用 %s 创建默认初始版本", appID)

	// 应用目录
	appDir := models.GetAppUploadDir(UploadDir, appID)
	versionsDir := filepath.Join(appDir, "versions")

	// 创建版本目录
	versionId := "1.0.0"
	versionDir := filepath.Join(versionsDir, versionId)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		log.Printf("创建初始版本目录失败: %v", err)
		return
	}

	// 检查是否已存在版本文件
	zipPath := filepath.Join(versionDir, "update.zip")
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		log.Printf("版本文件已存在，跳过创建空文件")
	} else {
		// 创建空的更新文件
		emptyFile, err := os.Create(zipPath)
		if err != nil {
			log.Printf("创建初始版本文件失败: %v", err)
			return
		}
		emptyFile.Close()
		log.Printf("已创建空的初始版本文件")
	}

	// 获取文件信息
	fileInfo, err := os.Stat(zipPath)
	if err != nil {
		log.Printf("获取文件信息失败: %v", err)
		return
	}

	// 创建初始版本信息
	initialVersion := models.Version{
		ID:          versionId,
		Name:        "初始版本",
		Description: "系统初始版本",
		FilePath:    filepath.Join("versions", versionId, "update.zip"),
		FileSize:    fileInfo.Size(),
		CreatedAt:   time.Now(),
		Force:       false,
	}

	versionList := &models.VersionList{
		Versions:      []models.Version{initialVersion},
		LatestVersion: versionId,
	}

	// 保存版本信息
	versionJsonPath := models.GetAppVersionsJsonPath(UploadDir, appID)
	if err := models.SaveVersions(versionList, versionJsonPath); err != nil {
		log.Printf("保存版本信息失败: %v", err)
		return
	}

	log.Printf("应用 %s 初始版本创建成功", appID)
}

// CreateVersion 创建新版本
func CreateVersion(c *gin.Context) {
	appID := c.Param("app_id")

	// 验证应用是否存在
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	if _, exists := models.GetApp(appList, appID); !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 解析表单
	err = c.Request.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法解析表单"})
		return
	}

	// 获取表单数据
	versionID := c.PostForm("version_id")
	name := c.PostForm("name")
	description := c.PostForm("description")
	forceUpdate := c.PostForm("force") == "true"

	// 验证版本ID
	if versionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "版本ID不能为空"})
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上传文件失败"})
		return
	}
	defer file.Close()

	// 检查文件类型
	if !strings.HasSuffix(header.Filename, ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只接受ZIP文件"})
		return
	}

	// 应用的版本目录
	appDir := models.GetAppUploadDir(UploadDir, appID)
	versionDir := filepath.Join(appDir, "versions", versionID)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建版本目录"})
		return
	}

	// 保存文件
	filePath := filepath.Join(versionDir, "update.zip")
	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建文件"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存文件"})
		return
	}

	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取文件信息"})
		return
	}

	// 加载现有版本列表
	versionJsonPath := models.GetAppVersionsJsonPath(UploadDir, appID)
	versionList, err := models.LoadVersions(versionJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载版本列表"})
		return
	}

	// 创建新版本信息
	newVersion := models.Version{
		ID:          versionID,
		Name:        name,
		Description: description,
		FilePath:    filepath.Join("versions", versionID, "update.zip"),
		FileSize:    fileInfo.Size(),
		CreatedAt:   time.Now(),
		Force:       forceUpdate,
	}

	// 添加到版本列表
	versionList = models.AddVersion(versionList, newVersion)

	// 保存版本列表
	if err := models.SaveVersions(versionList, versionJsonPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存版本列表"})
		return
	}

	log.Printf("应用 %s 已创建新版本: %s", appID, versionID)
	c.JSON(http.StatusOK, gin.H{"message": "版本创建成功", "version": newVersion})
}

// ListVersions 列出所有版本
func ListVersions(c *gin.Context) {
	appID := c.Param("app_id")

	// 验证应用是否存在
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	if _, exists := models.GetApp(appList, appID); !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	versionJsonPath := models.GetAppVersionsJsonPath(UploadDir, appID)
	versionList, err := models.LoadVersions(versionJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载版本列表"})
		return
	}

	c.JSON(http.StatusOK, versionList)
}

// CheckUpdate 检查更新
func CheckUpdate(c *gin.Context) {
	appID := c.Param("app_id")

	// 验证应用是否存在
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	if _, exists := models.GetApp(appList, appID); !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	clientVersion := c.Query("version")
	if clientVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少版本参数"})
		return
	}

	// 加载版本列表
	versionJsonPath := models.GetAppVersionsJsonPath(UploadDir, appID)
	versionList, err := models.LoadVersions(versionJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载版本列表"})
		return
	}

	// 如果没有版本
	if len(versionList.Versions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"hasUpdate": false,
			"message":   "没有可用更新",
		})
		return
	}

	// 获取所有版本
	versions := versionList.Versions

	// 最新版本
	latestVersion := versions[len(versions)-1]

	// 比较版本号
	versionCompare := compareVersions(clientVersion, latestVersion.ID)
	hasUpdate := versionCompare < 0 || latestVersion.Force

	if !hasUpdate {
		// 没有更新
		c.JSON(http.StatusOK, gin.H{
			"hasUpdate": false,
			"message":   "您的版本已是最新",
		})
		return
	}

	// 渐进式更新：找到客户端应该更新到的下一个版本
	var nextVersion models.Version
	var updatePath []models.Version

	// 查找客户端当前版本在版本列表中的位置
	clientVersionIndex := -1
	for i, v := range versions {
		if v.ID == clientVersion {
			clientVersionIndex = i
			break
		}
	}

	// 如果找不到客户端版本，或者客户端版本是最后一个版本，则返回一个空的更新路径
	if clientVersionIndex == -1 {
		// 客户端版本不在列表中，找到列表中第一个比客户端版本新的版本
		for _, v := range versions {
			if compareVersions(clientVersion, v.ID) < 0 {
				updatePath = append(updatePath, v)
				break
			}
		}
	} else if clientVersionIndex < len(versions)-1 {
		// 客户端版本在列表中，且不是最新版本，添加下一个版本
		nextVersion = versions[clientVersionIndex+1]
		updatePath = append(updatePath, nextVersion)
	}

	// 如果强制更新，直接返回最新版本
	if latestVersion.Force && len(updatePath) == 0 {
		updatePath = append(updatePath, latestVersion)
	}

	// 如果没有找到合适的更新路径，则返回没有更新
	if len(updatePath) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"hasUpdate": false,
			"message":   "没有可用更新",
		})
		return
	}

	// 返回客户端应该更新的下一个版本
	nextUpdateVersion := updatePath[0]

	c.JSON(http.StatusOK, gin.H{
		"hasUpdate":      true,
		"isProgressive":  true,
		"appID":          appID,
		"currentVersion": clientVersion,
		"latestVersion":  latestVersion.ID,
		"nextVersion":    nextUpdateVersion.ID,
		"updateUrl":      fmt.Sprintf("/api/apps/%s/download/%s/update.zip", appID, nextUpdateVersion.ID),
		"updateInfo":     nextUpdateVersion,
		"hasMoreUpdates": compareVersions(nextUpdateVersion.ID, latestVersion.ID) < 0,
	})
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) {
	appID := c.Param("app_id")
	version := c.Param("version")
	filename := c.Param("filename")

	// 验证应用是否存在
	appList, err := models.LoadApps(AppsJsonPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法加载应用列表"})
		return
	}

	if _, exists := models.GetApp(appList, appID); !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "应用不存在"})
		return
	}

	// 构造文件路径
	appDir := models.GetAppUploadDir(UploadDir, appID)
	filePath := filepath.Join(appDir, "versions", version, filename)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	// 提供文件下载
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}

// 比较版本号，返回：
// -1 如果 v1 < v2
//
//	0 如果 v1 == v2
//	1 如果 v1 > v2
func compareVersions(v1, v2 string) int {
	v1Parts := strings.Split(v1, ".")
	v2Parts := strings.Split(v2, ".")

	// 获取最大长度
	maxLen := len(v1Parts)
	if len(v2Parts) > maxLen {
		maxLen = len(v2Parts)
	}

	// 补齐短的版本号
	for len(v1Parts) < maxLen {
		v1Parts = append(v1Parts, "0")
	}
	for len(v2Parts) < maxLen {
		v2Parts = append(v2Parts, "0")
	}

	// 逐段比较
	for i := 0; i < maxLen; i++ {
		num1, _ := strconv.Atoi(v1Parts[i])
		num2, _ := strconv.Atoi(v2Parts[i])

		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
	}

	return 0
}
