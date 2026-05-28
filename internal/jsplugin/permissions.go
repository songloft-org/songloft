package jsplugin

import (
	"fmt"
	"strings"
)

// Permission 权限常量
//
// 执行层（runtime）权限 = 插件可申明的细粒度权限。
// 声明层允许题外子集： songs.* / playlists.* 仅作为“一把梭”通配符糖，
// 以配合 CheckPermission 的前缀匹配实现。
const (
	PermStorage        = "storage"         // 持久化存储
	PermSongsRead      = "songs.read"      // 读取歌曲
	PermSongsWrite     = "songs.write"     // 修改歌曲
	PermPlaylistsRead  = "playlists.read"  // 读取歌单
	PermPlaylistsWrite = "playlists.write" // 修改歌单
	PermInterPlugin    = "inter-plugin"    // 插件间通信
	PermCommand        = "command"         // 执行命令
	PermJSEnv          = "jsenv"           // 创建/执行子 JS 环境（songloft.jsenv.*）
)

// AllPermissions 所有合法权限列表（声明层白名单）。
// 包含两个通配符糖 songs.* / playlists.*：仅当作声明时的便捷写法，
// 实际走 CheckPermission 的前缀匹配，runtime 层的 action 仍会被
// extractPermFromAction 映射为细粒度权限。
var AllPermissions = []string{
	PermStorage,
	PermSongsRead, PermSongsWrite,
	PermPlaylistsRead, PermPlaylistsWrite,
	PermInterPlugin, PermCommand,
	PermJSEnv,
	// 通配符糖
	"songs.*",
	"playlists.*",
}

// CheckPermission 检查插件是否拥有指定权限
// 支持通配符匹配：如 "playlists.*" 匹配 "playlists.read"
func CheckPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == required {
			return true
		}
		// 通配符匹配：如果权限以 ".*" 结尾，则匹配相同前缀的所有子权限
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			if strings.HasPrefix(required, prefix+".") || required == prefix {
				return true
			}
		}
	}
	return false
}

// ValidatePermissions 验证权限列表中的每个权限是否合法
func ValidatePermissions(permissions []string) error {
	valid := make(map[string]bool, len(AllPermissions))
	for _, p := range AllPermissions {
		valid[p] = true
	}
	for _, p := range permissions {
		if !valid[p] {
			return fmt.Errorf("unknown permission: %q", p)
		}
	}
	return nil
}
