package config

import (
	"io/ioutil"
	"time"

	"github.com/quanxiang-cloud/dispatcher/pkg/misc/kafka"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/logger"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/mysql2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/redis2"

	"gopkg.in/yaml.v2"
)

// Conf 全局配置文件
var Conf *Config

// DefaultPath 默认配置路径
var DefaultPath = "./configs/config.yml"

// Config 配置文件
type Config struct {
	Port         string        `yaml:"port"`
	Model        string        `yaml:"model"`
	ProcessorNum int           `yaml:"processorNum"`
	SyncChannel  string        `yaml:"syncChannel"`
	HandOut      HandOut       `yaml:"handout"`
	Log          logger.Config `yaml:"log"`
	Kafka        kafka.Config  `yaml:"kafka"`
	Mysql        mysql2.Config `yaml:"mysql"`
	Redis        redis2.Config `yaml:"redis"`
}

// NewConfig 获取配置配置
func NewConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath
	}

	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(file, &Conf)
	if err != nil {
		return nil, err
	}

	return Conf, nil
}

// HandOut hand out
type HandOut struct {
	Deadline     time.Duration
	DialTimeout  time.Duration
	MaxIdleConns int
}
