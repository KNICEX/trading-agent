package binance

import (
	"context"
	"github.com/adshao/go-binance/v2"
	"github.com/spf13/viper"
	"testing"
	"time"
)

func initClient() *binance.Client {
	viper.AddConfigPath("../../../../config")
	viper.SetConfigName("config.dev")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	type Config struct {
		ApiSecret string `mapstructure:"api_secret"`
		ApiKey    string `mapstructure:"api_key"`
	}
	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(err)
	}
	return binance.NewClient(config.ApiKey, config.ApiSecret)
}

func TestGetKLine(t *testing.T) {
	cli := initClient()
	symbol := "BTCUSDT"
	interval := "15m"
	limit := 1000
	kline, err := cli.NewKlinesService().Symbol(symbol).
		StartTime(time.Now().Add(-time.Hour * 24 * 5).UnixMilli()).EndTime(time.Now().UnixMilli()).
		Interval(interval).Limit(limit).Do(context.Background())
	if err != nil {
		t.Errorf("Error getting kline: %v", err)
		return
	}
	for _, k := range kline {
		t.Logf("Kline: %+v", k)
	}
}
