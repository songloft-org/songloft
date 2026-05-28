//go:build !dev
// +build !dev

package app

// registerSwagger 空实现，生产环境不注册Swagger路由
func (a *App) registerSwagger() {
	// 生产环境不启用Swagger
}
