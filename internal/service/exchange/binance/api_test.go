package binance

import (
	"testing"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/spf13/viper"
)

func initClient(t *testing.T) *futures.Client {
	viper.AddConfigPath("../../../../config")
	viper.SetConfigName("config.dev")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	type Config struct {
		Exchange map[string]struct {
			ApiKey    string `mapstructure:"api_key"`
			ApiSecret string `mapstructure:"api_secret"`
		} `mapstructure:"exchange"`
	}
	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(err)
	}
	return futures.NewClient(config.Exchange["binance"].ApiKey, config.Exchange["binance"].ApiSecret)
}
