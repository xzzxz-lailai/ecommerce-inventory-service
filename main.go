package main

import (
	commonetcd "ecommerce/etcd"
	"inventory-service/config"
	"inventory-service/router"
)

func main() {
	// 初始化配置（包含数据库初始化）。
	cfg := config.InitConfig()

	// 初始化 etcd，并注册库存服务。
	if err := commonetcd.InitEtcd(config.Cfg.Etcd.Host); err != nil {
		panic(err)
	}
	defer commonetcd.CloseEtcd()

	if err := commonetcd.RegisterService(
		config.Cfg.Etcd.ServerName,
		config.Cfg.Etcd.ServeAddress,
	); err != nil {
		panic(err)
	}

	// 初始化业务路由。
	r := router.Router()
	if err := r.Run(cfg.Server.Port); err != nil {
		panic(err)
	}
}
