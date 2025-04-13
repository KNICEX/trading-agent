package main

import (
	"context"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/repo"
	"github.com/KNICEX/trading-agent/internal/service/exchange/binance"
	"github.com/KNICEX/trading-agent/internal/service/monitor"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"github.com/KNICEX/trading-agent/ioc"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"time"
)

func initViper() {

	// --config=./config/xxx.yaml
	file := pflag.String("config", "./config/config.dev.yaml", "specify config file")

	viper.SetConfigFile(*file)
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s \n", err))
	}

}

func main() {
	initViper()

	db := ioc.InitDB()
	//geminiCli := ioc.InitGeminiCli()
	//llmSvc := gemini.NewService(geminiCli)
	bian := ioc.InitBinanceCli()

	symbolSvc := binance.NewSymbolService(bian)
	marketSvc := binance.NewMarketService(bian)

	if err := repo.InitTables(db); err != nil {
		panic(err)
	}
	abnormalRepo := repo.NewAbnormalRepo(db)
	abnormalAnalyzer := strategy.NewRuleBasedAnalyzer()

	abnormalMonitor := monitor.NewAbnormalMonitor(abnormalAnalyzer, abnormalRepo, symbolSvc, marketSvc)
	task := monitor.NewAbnormalMonitorTask(abnormalMonitor, symbolSvc)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	if err := task.Run(ctx); err != nil {
		panic(err)
	}
}
