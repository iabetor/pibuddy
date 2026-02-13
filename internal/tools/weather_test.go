package tools

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWeatherTool_Name(t *testing.T) {
	tool := NewWeatherTool(WeatherConfig{APIKey: "test"})
	if tool.Name() != "get_weather" {
		t.Errorf("expected name 'get_weather', got %q", tool.Name())
	}
}

func TestWeatherTool_EmptyCity(t *testing.T) {
	tool := NewWeatherTool(WeatherConfig{APIKey: "test"})
	args, _ := json.Marshal(weatherArgs{City: ""})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for empty city")
	}
}

func TestWeatherTool_InvalidJSON(t *testing.T) {
	tool := NewWeatherTool(WeatherConfig{APIKey: "test"})
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestWeatherTool_WithMockServer uses httptest to simulate QWeather API (API Key mode)
func TestWeatherTool_WithMockServer(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/geo/v2/city/lookup", func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-QW-Api-Key")
		if apiKey != "testkey" {
			t.Errorf("expected X-QW-Api-Key header 'testkey', got %q", apiKey)
		}
		location := r.URL.Query().Get("location")
		if location == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := `{"code":"200","location":[{"name":"北京","id":"101010100","adm1":"北京","adm2":"北京","country":"中国"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	mux.HandleFunc("/v7/weather/now", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"code":"200","now":{"obsTime":"2026-02-13T14:00+08:00","temp":"5","feelsLike":"1","text":"晴","windDir":"北风","windScale":"3","humidity":"30","vis":"25"}}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	mux.HandleFunc("/v7/weather/3d", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"code":"200","daily":[{"fxDate":"2026-02-13","tempMax":"8","tempMin":"-2","textDay":"晴","textNight":"多云","windDirDay":"北风","windScaleDay":"3-4"},{"fxDate":"2026-02-14","tempMax":"10","tempMin":"0","textDay":"多云","textNight":"阴","windDirDay":"南风","windScaleDay":"2-3"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "https://")

	tool := &WeatherTool{
		apiKey:  "testkey",
		apiHost: host,
		client:  server.Client(),
	}

	args, _ := json.Marshal(weatherArgs{City: "北京"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "北京") {
		t.Errorf("result should contain city name, got %q", result)
	}
	if !strings.Contains(result, "晴") {
		t.Errorf("result should contain weather text '晴', got %q", result)
	}
	if !strings.Contains(result, "5°C") {
		t.Errorf("result should contain temp '5°C', got %q", result)
	}
	if !strings.Contains(result, "预报") {
		t.Errorf("result should contain forecast section, got %q", result)
	}

	t.Logf("Weather result:\n%s", result)
}

// TestWeatherTool_JWTAuth tests JWT authentication mode
func TestWeatherTool_JWTAuth(t *testing.T) {
	// Generate test Ed25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Write private key to temp file
	privBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	tmpFile, err := os.CreateTemp("", "ed25519-test-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(privPEM); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}
	tmpFile.Close()

	_ = pubKey // used for verification in mock server below

	mux := http.NewServeMux()

	mux.HandleFunc("/geo/v2/city/lookup", func(w http.ResponseWriter, r *http.Request) {
		// Verify JWT Bearer token is present
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Authorization: Bearer header, got %q", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Should NOT have X-QW-Api-Key
		if r.Header.Get("X-QW-Api-Key") != "" {
			t.Error("should not send X-QW-Api-Key when using JWT")
		}

		resp := `{"code":"200","location":[{"name":"深圳","id":"101280601","adm1":"广东","adm2":"深圳","country":"中国"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	mux.HandleFunc("/v7/weather/now", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"code":"200","now":{"obsTime":"2026-02-13T14:00+08:00","temp":"18","feelsLike":"16","text":"多云","windDir":"东南风","windScale":"2","humidity":"65","vis":"20"}}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	mux.HandleFunc("/v7/weather/3d", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"code":"200","daily":[{"fxDate":"2026-02-13","tempMax":"20","tempMin":"12","textDay":"多云","textNight":"晴","windDirDay":"东南风","windScaleDay":"2-3"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "https://")

	tool := NewWeatherTool(WeatherConfig{
		APIHost:        host,
		CredentialID:   "test-cred-id",
		ProjectID:      "test-proj-id",
		PrivateKeyPath: tmpFile.Name(),
	})
	// Override the HTTP client to use the test server's TLS client
	tool.client = server.Client()

	if !tool.useJWT {
		t.Fatal("expected JWT mode to be enabled")
	}

	args, _ := json.Marshal(weatherArgs{City: "深圳"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "深圳") {
		t.Errorf("result should contain '深圳', got %q", result)
	}
	if !strings.Contains(result, "多云") {
		t.Errorf("result should contain '多云', got %q", result)
	}

	t.Logf("JWT Weather result:\n%s", result)
}

// TestWeatherTool_JWTTokenCaching verifies token is cached
func TestWeatherTool_JWTTokenCaching(t *testing.T) {
	_, privKey, _ := ed25519.GenerateKey(nil)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	tmpFile, _ := os.CreateTemp("", "ed25519-test-*.pem")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(privPEM)
	tmpFile.Close()

	tool := NewWeatherTool(WeatherConfig{
		CredentialID:   "cred",
		ProjectID:      "proj",
		PrivateKeyPath: tmpFile.Name(),
	})

	if !tool.useJWT {
		t.Fatal("expected JWT mode")
	}

	token1, err := tool.getToken()
	if err != nil {
		t.Fatalf("first getToken failed: %v", err)
	}

	token2, err := tool.getToken()
	if err != nil {
		t.Fatalf("second getToken failed: %v", err)
	}

	if token1 != token2 {
		t.Error("expected cached token to be reused")
	}

	// Verify token has 3 parts (header.payload.signature)
	parts := strings.Split(token1, ".")
	if len(parts) != 3 {
		t.Errorf("JWT should have 3 parts, got %d", len(parts))
	}
}

// TestWeatherTool_JWTFallbackOnBadKey tests graceful fallback when key is invalid
func TestWeatherTool_JWTFallbackOnBadKey(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "bad-key-*.pem")
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("not a valid key")
	tmpFile.Close()

	tool := NewWeatherTool(WeatherConfig{
		APIKey:         "fallback-key",
		CredentialID:   "cred",
		ProjectID:      "proj",
		PrivateKeyPath: tmpFile.Name(),
	})

	if tool.useJWT {
		t.Error("should have fallen back to API Key mode on bad key")
	}
	if tool.apiKey != "fallback-key" {
		t.Errorf("apiKey should be 'fallback-key', got %q", tool.apiKey)
	}
}

// TestWeatherTool_CityNotFound tests when city is not found
func TestWeatherTool_CityNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/geo/v2/city/lookup", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"code": "404", "location": []}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "https://")
	tool := &WeatherTool{
		apiKey:  "testkey",
		apiHost: host,
		client:  server.Client(),
	}

	args, _ := json.Marshal(weatherArgs{City: "不存在的城市"})
	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for non-existent city")
	}
	if !strings.Contains(err.Error(), "未找到城市") {
		t.Errorf("error should mention '未找到城市', got %q", err.Error())
	}
}

// TestWeatherTool_GeoHost verifies geoHost returns same as apiHost
func TestWeatherTool_GeoHost(t *testing.T) {
	tool := NewWeatherTool(WeatherConfig{
		APIKey:  "test",
		APIHost: "custom.host.com",
	})
	if tool.geoHost() != "custom.host.com" {
		t.Errorf("geoHost should return apiHost, got %q", tool.geoHost())
	}
}

func TestWeatherTool_DefaultHost(t *testing.T) {
	tool := NewWeatherTool(WeatherConfig{APIKey: "test"})
	if tool.apiHost != "devapi.qweather.com" {
		t.Errorf("default host should be devapi.qweather.com, got %q", tool.apiHost)
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b", "c"}, "a\nb\nc"},
	}
	for _, tt := range tests {
		result := joinLines(tt.input)
		if result != tt.expected {
			t.Errorf("joinLines(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateJWT(t *testing.T) {
	_, privKey, _ := ed25519.GenerateKey(nil)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	tmpFile, _ := os.CreateTemp("", "ed25519-test-*.pem")
	defer os.Remove(tmpFile.Name())
	tmpFile.Write(privPEM)
	tmpFile.Close()

	tool := NewWeatherTool(WeatherConfig{
		CredentialID:   "MY_CRED",
		ProjectID:      "MY_PROJ",
		PrivateKeyPath: tmpFile.Name(),
	})

	token, err := tool.generateJWT()
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT should have 3 parts, got %d", len(parts))
	}

	// Decode header
	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		t.Fatalf("failed to decode header: %v", err)
	}
	var header map[string]string
	json.Unmarshal(headerJSON, &header)
	if header["alg"] != "EdDSA" {
		t.Errorf("expected alg EdDSA, got %s", header["alg"])
	}
	if header["kid"] != "MY_CRED" {
		t.Errorf("expected kid MY_CRED, got %s", header["kid"])
	}

	// Decode payload
	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	var payload map[string]interface{}
	json.Unmarshal(payloadJSON, &payload)
	if payload["sub"] != "MY_PROJ" {
		t.Errorf("expected sub MY_PROJ, got %v", payload["sub"])
	}

	t.Logf("JWT token: %s...", token[:50])
}

// base64URLDecode is a test helper for decoding base64url.
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
