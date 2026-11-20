package config

// AppConfig 应用配置
type AppConfig struct {
	Port                    string
	DBPath                  string
	Username                string
	Password                string
	BasePath                string
	UsingDefaultCredentials bool
	MusicDir                string // 移动端传入的音乐目录（非空时覆盖 DB 中的 music_path 默认值）
	FFmpegPath              string // Bundled executable, supplied by the host
	FFprobePath             string // Bundled executable, supplied by the host
	ParentWatchStdin        bool   // 桌面宿主通过继承的 stdin 管道声明父进程生命周期
}

// NewAppConfig 创建 AppConfig（供 mobile 包等非 CLI 入口使用，不依赖 flag.Parse）
func NewAppConfig(port, dbPath, username, password, basePath string) *AppConfig {
	usingDefault := username == "" || password == ""
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "admin"
	}
	return &AppConfig{
		Port:                    port,
		DBPath:                  dbPath,
		Username:                username,
		Password:                password,
		BasePath:                basePath,
		UsingDefaultCredentials: usingDefault,
	}
}
