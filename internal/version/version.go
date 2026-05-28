package version

// 这些变量将在编译时通过 -ldflags 注入
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	BuildType = "" // 构建类型，"full" 或 ""（默认 lite）
)

// GetVersion 返回完整的版本信息
func GetVersion() string {
	return Version
}

// GetFullVersion 返回包含 Git 提交和构建时间的完整版本信息
func GetFullVersion() string {
	s := Version + " (commit: " + GitCommit + ", built: " + BuildTime
	if BuildType != "" {
		s += ", type: " + BuildType
	}
	s += ")"
	return s
}
