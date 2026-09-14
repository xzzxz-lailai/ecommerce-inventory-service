package config

import (
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type MySQLConfig struct {
	Host   string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	User   string `mapstructure:"user"`
	Pass   string `mapstructure:"password"`
	DBName string `mapstructure:"dbname"`
}

type EtcdConfig struct {
	Host         string `mapstructure:"host"`
	ServerName   string `mapstructure:"server_name"`
	ServeAddress string `mapstructure:"address"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"` // 需要与 user-service 保持一致。
	Expire int    `mapstructure:"expire"` // 有效期，单位为小时。
}

type Config struct {
	Server ServerConfig
	MySQL  MySQLConfig
	Etcd   EtcdConfig
	JWT    JWTConfig
}

var Cfg Config

func InitConfig() *Config {
	nacosConfig, err := GetNacosConfig()
	if err != nil {
		panic(err)
	}

	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(strings.NewReader(nacosConfig)); err != nil {
		panic(err)
	}
	if err := viper.Unmarshal(&Cfg); err != nil {
		panic(err)
	}

	if _, err := NewMySQL(); err != nil {
		panic(err)
	}

	return &Cfg
}
