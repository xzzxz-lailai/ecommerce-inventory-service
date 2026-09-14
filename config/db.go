package config

import (
	"fmt"

	"inventory-service/global"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func NewMySQL() (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		Cfg.MySQL.User,
		Cfg.MySQL.Pass,
		Cfg.MySQL.Host,
		Cfg.MySQL.Port,
		Cfg.MySQL.DBName,
	)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, err
	}

	global.DB = db
	fmt.Println("✅ Mysql 数据库连接成功")

	return db, nil
}
