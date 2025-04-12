package ioc

import (
	"context"
	"github.com/google/generative-ai-go/genai"
	"github.com/spf13/viper"
	"google.golang.org/api/option"
)

func InitGeminiCli() *genai.Client {
	type Config struct {
		ApiKey []string `mapstructure:"api_key"`
	}

	var cfg Config
	if err := viper.UnmarshalKey("llm.gemini", &cfg); err != nil {
		panic(err)
	}

	if len(cfg.ApiKey) == 0 {
		panic("no gemini api key set")
	}

	cli, err := genai.NewClient(context.Background(), option.WithAPIKey(cfg.ApiKey[0]))
	if err != nil {
		panic(err)
	}
	return cli
}
