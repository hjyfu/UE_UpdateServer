package models

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// App 表示一个应用项目
type App struct {
	ID          string    `json:"id"`          // 应用ID，唯一标识
	Name        string    `json:"name"`        // 应用名称
	Description string    `json:"description"` // 应用描述
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"`   // 更新时间
}

// AppList 表示应用列表
type AppList struct {
	Apps []App `json:"apps"` // 应用列表
}

// LoadApps 从文件加载应用信息
func LoadApps(filePath string) (*AppList, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &AppList{
			Apps: []App{},
		}, nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var appList AppList
	err = json.Unmarshal(data, &appList)
	if err != nil {
		return nil, err
	}

	return &appList, nil
}

// SaveApps 保存应用信息到文件
func SaveApps(appList *AppList, filePath string) error {
	data, err := json.MarshalIndent(appList, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, data, 0644)
}

// AddApp 添加新应用
func AddApp(appList *AppList, app App) *AppList {
	// 检查应用ID是否已存在
	for i, existingApp := range appList.Apps {
		if existingApp.ID == app.ID {
			// 更新已存在的应用
			appList.Apps[i] = app
			return appList
		}
	}

	// 添加新应用
	appList.Apps = append(appList.Apps, app)
	return appList
}

// GetApp 根据ID获取应用
func GetApp(appList *AppList, appID string) (App, bool) {
	for _, app := range appList.Apps {
		if app.ID == appID {
			return app, true
		}
	}
	return App{}, false
}

// DeleteApp 删除应用
func DeleteApp(appList *AppList, appID string) *AppList {
	for i, app := range appList.Apps {
		if app.ID == appID {
			// 删除该应用
			appList.Apps = append(appList.Apps[:i], appList.Apps[i+1:]...)
			break
		}
	}
	return appList
}

// GetAppUploadDir 获取应用上传目录
func GetAppUploadDir(baseUploadDir string, appID string) string {
	return filepath.Join(baseUploadDir, "apps", appID)
}

// GetAppVersionsJsonPath 获取应用版本信息文件路径
func GetAppVersionsJsonPath(baseUploadDir string, appID string) string {
	appDir := GetAppUploadDir(baseUploadDir, appID)
	return filepath.Join(appDir, "versions.json")
}

// CreateAppDirectories 创建应用所需的目录
func CreateAppDirectories(baseUploadDir string, appID string) error {
	appDir := GetAppUploadDir(baseUploadDir, appID)
	versionsDir := filepath.Join(appDir, "versions")

	// 创建应用目录
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return err
	}

	// 创建版本目录
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return err
	}

	return nil
}
