package ioc

import (
	"github.com/adshao/go-binance/v2"
	"github.com/spf13/viper"
)

func InitBinanceCli() *binance.Client {
	type Config struct {
		ApiKey    string `mapstructure:"api_key"`
		ApiSecret string `mapstructure:"api_secret"`
	}

	var cfg Config
	if err := viper.UnmarshalKey("cex.binance", &cfg); err != nil {
		panic(err)
	}

	return binance.NewClient(cfg.ApiKey, cfg.ApiSecret)
}
