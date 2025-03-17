package models

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// Version 表示一个版本信息
type Version struct {
	ID          string    `json:"id"`          // 版本ID
	Name        string    `json:"name"`        // 版本名称
	Description string    `json:"description"` // 版本描述
	FilePath    string    `json:"filePath"`    // 版本文件路径
	FileSize    int64     `json:"fileSize"`    // 文件大小
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	Force       bool      `json:"force"`       // 是否强制更新
}

// VersionList 表示版本列表
type VersionList struct {
	Versions      []Version `json:"versions"`      // 版本列表
	LatestVersion string    `json:"latestVersion"` // 最新版本
}

// LoadVersions 从文件加载版本信息
func LoadVersions(filePath string) (*VersionList, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &VersionList{
			Versions:      []Version{},
			LatestVersion: "",
		}, nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var versionList VersionList
	err = json.Unmarshal(data, &versionList)
	if err != nil {
		return nil, err
	}

	return &versionList, nil
}

// SaveVersions 保存版本信息到文件
func SaveVersions(versionList *VersionList, filePath string) error {
	data, err := json.MarshalIndent(versionList, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filePath, data, 0644)
}

// AddVersion 添加新版本
func AddVersion(versionList *VersionList, version Version) *VersionList {
	versionList.Versions = append(versionList.Versions, version)
	versionList.LatestVersion = version.ID
	return versionList
}

// CreateInitialVersion 创建初始版本
func CreateInitialVersion(uploadsDir string, initialZip string) (*VersionList, error) {
	// 创建版本目录
	versionId := "1.0.0"
	versionDir := filepath.Join(uploadsDir, "versions", versionId)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return nil, err
	}

	// 假设初始zip文件已经存在
	zipPath := filepath.Join(versionDir, "update.zip")

	// 如果提供了初始zip文件，则复制它
	if initialZip != "" {
		if err := copyFile(initialZip, zipPath); err != nil {
			return nil, err
		}
	} else {
		// 创建空文件作为占位符
		emptyFile, err := os.Create(zipPath)
		if err != nil {
			return nil, err
		}
		emptyFile.Close()
	}

	fileInfo, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}

	// 创建初始版本信息
	initialVersion := Version{
		ID:          versionId,
		Name:        "初始版本",
		Description: "系统初始版本",
		FilePath:    filepath.Join("versions", versionId, "update.zip"),
		FileSize:    fileInfo.Size(),
		CreatedAt:   time.Now(),
		Force:       false,
	}

	versionList := &VersionList{
		Versions:      []Version{initialVersion},
		LatestVersion: versionId,
	}

	// 保存版本信息
	if err := SaveVersions(versionList, filepath.Join(uploadsDir, "versions.json")); err != nil {
		return nil, err
	}

	return versionList, nil
}

// 辅助函数：复制文件
func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0644)
}
