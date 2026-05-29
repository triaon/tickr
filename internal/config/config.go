package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ExchangeConfig struct {
	Enabled         bool   `yaml:"enabled"`
	BaseURL         string `yaml:"base_url,omitempty"`
	SpotBaseURL     string `yaml:"spot_base_url,omitempty"`
	FuturesBaseURL  string `yaml:"futures_base_url,omitempty"`
}

type OutputConfig struct {
	DefaultFormat string `yaml:"default_format"`
	IncludeRaw    bool   `yaml:"include_raw"`
}

type TradingViewConfig struct {
	SuffixPerp string `yaml:"suffix_perp"`
}

type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type Config struct {
	Exchanges   map[string]ExchangeConfig `yaml:"exchanges"`
	Output      OutputConfig              `yaml:"output"`
	TradingView TradingViewConfig         `yaml:"tradingview"`
	Telegram    TelegramConfig            `yaml:"telegram"`
}

// Default returns a Config with built-in URLs and sensible defaults.
func Default() *Config {
	return &Config{
		Exchanges: map[string]ExchangeConfig{
			"binance": {Enabled: true, SpotBaseURL: "https://api.binance.com", FuturesBaseURL: "https://fapi.binance.com"},
			"bybit":   {Enabled: true, BaseURL: "https://api.bybit.com"},
			"mexc":    {Enabled: true, SpotBaseURL: "https://api.mexc.com", FuturesBaseURL: "https://contract.mexc.com"},
			"bingx":   {Enabled: true, BaseURL: "https://open-api.bingx.com"},
		},
		Output:      OutputConfig{DefaultFormat: "json", IncludeRaw: false},
		TradingView: TradingViewConfig{SuffixPerp: ".P"},
		Telegram:    TelegramConfig{Enabled: false},
	}
}

// Load reads a YAML config file. If path is empty or file does not exist, returns defaults.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.merge(&loaded)
	return cfg, nil
}

func (c *Config) merge(o *Config) {
	for name, ex := range o.Exchanges {
		base := c.Exchanges[name]
		if ex.BaseURL != "" {
			base.BaseURL = ex.BaseURL
		}
		if ex.SpotBaseURL != "" {
			base.SpotBaseURL = ex.SpotBaseURL
		}
		if ex.FuturesBaseURL != "" {
			base.FuturesBaseURL = ex.FuturesBaseURL
		}
		base.Enabled = ex.Enabled
		c.Exchanges[name] = base
	}
	if o.Output.DefaultFormat != "" {
		c.Output.DefaultFormat = o.Output.DefaultFormat
	}
	c.Output.IncludeRaw = o.Output.IncludeRaw
	if o.TradingView.SuffixPerp != "" {
		c.TradingView.SuffixPerp = o.TradingView.SuffixPerp
	}
	if o.Telegram.BotToken != "" {
		c.Telegram.BotToken = o.Telegram.BotToken
	}
	if o.Telegram.ChatID != "" {
		c.Telegram.ChatID = o.Telegram.ChatID
	}
	c.Telegram.Enabled = o.Telegram.Enabled
}
