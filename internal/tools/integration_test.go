//go:build integration

package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// These tests call real external APIs. Run with:
//   go test ./internal/tools/ -tags=integration -v -run TestIntegration

func getEnvOrSkip(t *testing.T, key string) string {
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: env %s not set", key)
	}
	return v
}

func getEnvDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func TestIntegration_Weather(t *testing.T) {
	tool := NewWeatherTool(WeatherConfig{
		APIHost: "q75ctvjkwx.re.qweatherapi.com",
		// JWT 认证：需要设置环境变量
		// PIBUDDY_QWEATHER_CREDENTIAL_ID, PIBUDDY_QWEATHER_PROJECT_ID
		CredentialID:   getEnvOrSkip(t, "PIBUDDY_QWEATHER_CREDENTIAL_ID"),
		ProjectID:      getEnvOrSkip(t, "PIBUDDY_QWEATHER_PROJECT_ID"),
		PrivateKeyPath: getEnvDefault("PIBUDDY_QWEATHER_PRIVATE_KEY_PATH", "./ed25519-private.pem"),
	})

	args, _ := json.Marshal(weatherArgs{City: "深圳"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("weather query failed: %v", err)
	}
	t.Logf("Weather result:\n%s", result)
}

func TestIntegration_Stock(t *testing.T) {
	tool := NewStockTool()

	tests := []struct {
		name string
		code string
	}{
		{"a股茅台", "600519"},
		{"a股平安", "000001"},
		{"a股带前缀", "sh600519"},
		{"港股腾讯", "00700"},
		{"港股阿里", "09988"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(stockArgs{Code: tt.code})
			result, err := tool.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("stock query failed for %s: %v", tt.code, err)
			}
			t.Logf("Stock result for %s: %s", tt.code, result)
		})
	}
}

func TestIntegration_News(t *testing.T) {
	tool := NewNewsTool()
	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("news query failed: %v", err)
	}
	t.Logf("News result:\n%s", result)
}
