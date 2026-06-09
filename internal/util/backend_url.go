package util

import (
	"os"

	"github.com/spf13/viper"
)

// GetBackendURL 获取后端URL，优先从环境变量获取，如果环境变量不存在则从配置获取
func GetBackendURL() string {
	// 优先从环境变量获取
	if backendURL := os.Getenv("BACKEND_URL"); backendURL != "" {
		return backendURL
	}
	// 从配置文件获取
	return viper.GetString("manager.backend_url")
}

// GetSidecarURL 获取 Sidecar URL，用于上行消息同步
// 如果未配置则返回空字符串
func GetSidecarURL() string {
	// 优先从环境变量获取
	if sidecarURL := os.Getenv("SIDECAR_URL"); sidecarURL != "" {
		return sidecarURL
	}
	// 从配置文件获取
	return viper.GetString("manager.sidecar_url")
}

