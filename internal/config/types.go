package config

// AppConfig 应用配置
type AppConfig struct {
	Port                    string
	DBPath                  string
	Username                string
	Password                string
	UsingDefaultCredentials bool
}
