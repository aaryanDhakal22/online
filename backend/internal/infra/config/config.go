package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

type Config struct {
	AppEnv        string
	LogLevel      zerolog.Level
	LogOutput     io.Writer
	Queuename     string
	LogStyle      string
	ServerPort    string
	RedisPort     string
	RedisPassword string
	AdminPassHash string
}

func NewConfig() *Config {
	config := Config{}
	config.AppEnv = getEnv("APP_ENV")
	config.LogLevel = getLogLevel()
	config.LogOutput = getLogOutput()
	config.LogStyle = getEnv("LOG_STYLE")
	config.ServerPort = getEnv("SERVER_PORT")
	if config.ServerPort == "" {
		config.ServerPort = "1323"
	}
	config.RedisPort = getEnv("REDIS_PORT")
	config.RedisPassword = getEnv("REDIS_PASS")
	config.AdminPassHash = getEnv("ADMIN_PASS_HASH")
	config.Queuename = getEnv("QUEUE_NAME")

	return &config
}

func generateLogFilePath() string {
	filePath := fmt.Sprintf("sinkLogs/app_%s.log", time.Now().Format("2006-01-02_15-04-05"))
	return filepath.Join(os.TempDir(), filePath)
}

func getLogOutput() io.Writer {
	text := os.Getenv("LOG_OUTPUT")
	if text == "file" {
		filePath := generateLogFilePath()
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Println("Error opening log file, defaulting to stderr")
			return os.Stderr
		}
		return file
	}
	if text == "standard" {
		return os.Stderr
	}
	fmt.Println("Invalid log output, defaulting to stderr")
	return os.Stderr
}

func getLogLevel() zerolog.Level {
	text := os.Getenv("LOG_LEVEL")
	logLevel, err := zerolog.ParseLevel(text)
	if err != nil {
		fmt.Println("Error parsing log level, defaulting to info")
		logLevel = zerolog.InfoLevel
	}
	return logLevel
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("Environment variable " + key + " is not set")
	}
	return value
}
