package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// L 是全局 logger 实例。
	L *zap.SugaredLogger
	// Z 是全局 zap.Logger 实例（用于需要性能的场景）。
	Z *zap.Logger
	// writer 用于关闭 lumberjack
	writer io.Writer
)

func init() {
	// 默认使用 info 级别，输出到 stderr。
	z, _ := zap.NewProduction()
	Z = z
	L = z.Sugar()
}

// Config 日志配置。
type Config struct {
	Level      string // 日志级别: debug, info, warn, error
	File       string // 日志文件路径，为空则只输出到控制台
	MaxSize    int    // 单个日志文件最大大小（MB）
	MaxBackups int    // 保留的旧日志文件最大数量
	MaxAge     int    // 保留旧日志文件的最大天数
}

// Init 根据配置初始化全局 logger。
func Init(cfg Config) error {
	var zapLevel zapcore.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info", "":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		return fmt.Errorf("不支持的日志级别: %s", cfg.Level)
	}

	// 编码器配置
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	// 确定输出目标
	var output io.Writer = os.Stderr
	if cfg.File != "" {
		// 确保日志目录存在
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0755); err != nil {
			return fmt.Errorf("创建日志目录失败: %w", err)
		}

		maxSize := cfg.MaxSize
		if maxSize <= 0 {
			maxSize = 64
		}
		maxBackups := cfg.MaxBackups
		if maxBackups <= 0 {
			maxBackups = 3
		}
		maxAge := cfg.MaxAge
		if maxAge <= 0 {
			maxAge = 7
		}

		// 同时输出到文件和控制台
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    maxSize,    // MB
			MaxBackups: maxBackups, // 保留旧文件数量
			MaxAge:     maxAge,     // 保留天数
			Compress:   true,       // 压缩旧文件
		}
		writer = fileWriter
		output = io.MultiWriter(os.Stderr, fileWriter)
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.AddSync(output),
		zapLevel,
	)

	Z = zap.New(core, zap.AddCallerSkip(1))
	L = Z.Sugar()
	return nil
}

// Sync 刷新缓冲区，应在程序退出前调用。
func Sync() {
	if Z != nil {
		_ = Z.Sync()
	}
}

// Debug 记录调试级别日志。
func Debug(msg string) { L.Debug(msg) }

// Debugf 记录格式化调试级别日志。
func Debugf(template string, args ...interface{}) { L.Debugf(template, args...) }

// Info 记录信息级别日志。
func Info(msg string) { L.Info(msg) }

// Infof 记录格式化信息级别日志。
func Infof(template string, args ...interface{}) { L.Infof(template, args...) }

// Warn 记录警告级别日志。
func Warn(msg string) { L.Warn(msg) }

// Warnf 记录格式化警告级别日志。
func Warnf(template string, args ...interface{}) { L.Warnf(template, args...) }

// Error 记录错误级别日志。
func Error(msg string) { L.Error(msg) }

// Errorf 记录格式化错误级别日志。
func Errorf(template string, args ...interface{}) { L.Errorf(template, args...) }
