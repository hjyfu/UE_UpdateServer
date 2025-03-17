package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// LogLevelType 日志级别类型
type LogLevelType int

const (
	DEBUG LogLevelType = iota
	INFO
	WARNING
	ERROR
	FATAL
)

var (
	logFile     *os.File
	LogLevel    = INFO
	initialized = false
)

// InitLogger 初始化日志系统
func InitLogger(logDir string, prefix string) error {
	if initialized {
		return nil
	}

	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("无法创建日志目录: %v", err)
	}

	// 生成日志文件名
	timestamp := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("server_%s.log", timestamp)
	logFilePath := filepath.Join(logDir, logFileName)

	// 打开日志文件
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %v", err)
	}

	logFile = f
	log.SetOutput(f)
	log.SetPrefix(prefix + " ")

	initialized = true

	log.Println("日志系统已初始化")
	return nil
}

// CloseLogger 关闭日志系统
func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

// Debug 记录调试级别日志
func Debug(format string, v ...interface{}) {
	if LogLevel <= DEBUG {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info 记录信息级别日志
func Info(format string, v ...interface{}) {
	if LogLevel <= INFO {
		log.Printf("[INFO] "+format, v...)
	}
}

// Warning 记录警告级别日志
func Warning(format string, v ...interface{}) {
	if LogLevel <= WARNING {
		log.Printf("[WARNING] "+format, v...)
	}
}

// Error 记录错误级别日志
func Error(format string, v ...interface{}) {
	if LogLevel <= ERROR {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Fatal 记录致命错误级别日志并终止程序
func Fatal(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}
