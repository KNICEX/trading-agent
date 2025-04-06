package main

import (
	"fmt"
	"github.com/spf13/viper"
)

func main() {
	initDev()
}

func initDev() {
	initViper("config.dev")
}

func initViper(name string) {
	viper.SetConfigName(name)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s \n", err))
	}
}
