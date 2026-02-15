package audio

import (
	"testing"
)

func TestInt16StereoToMonoFloat32(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []float32
	}{
		{
			name:  "转换立体声数据",
			input: []byte{0x00, 0x80, 0x00, 0x80, 0xFF, 0x7F, 0xFF, 0x7F}, // 两个立体声样本
			expected: []float32{
				// 0x8000 = -32768 (int16 最小值), 0x7FFF = 32767 (int16 最大值)
				(float32(-32768) + float32(-32768)) / 65536.0, // 左右声道都为 -32768
				(float32(32767) + float32(32767)) / 65536.0,   // 左右声道都为 32767
			},
		},
		{
			name:     "空输入",
			input:    []byte{},
			expected: nil,
		},
		{
			name:     "单声道样本（4字节）",
			input:    []byte{0x00, 0x00, 0x00, 0x00},
			expected: []float32{0.0},
		},
		{
			name:  "静音数据",
			input: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			expected: []float32{
				0.0,
				0.0,
			},
		},
		{
			name:  "最大音量",
			input: []byte{0xFF, 0x7F, 0xFF, 0x7F, 0x01, 0x80, 0x01, 0x80}, // 0x7FFF (正最大) 和 0x8001 (负最大)
			expected: []float32{
				(float32(0x7FFF) + float32(0x7FFF)) / 65536.0,
				(float32(-32767) + float32(-32767)) / 65536.0, // 0x8001 = -32767
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := int16StereoToMonoFloat32(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("结果长度错误: got %d, want %d", len(result), len(tt.expected))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("样本 %d 错误: got %f, want %f", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFloat32ToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected int // 期望的字节长度
	}{
		{
			name:     "转换 float32 数组",
			input:    []float32{0.5, -0.5, 0.0, 1.0},
			expected: 8, // 4 个样本 * 2 字节
		},
		{
			name:     "空输入",
			input:    []float32{},
			expected: 0,
		},
		{
			name:     "单样本",
			input:    []float32{0.5},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Float32ToBytes(tt.input)

			if len(result) != tt.expected {
				t.Errorf("字节长度错误: got %d, want %d", len(result), tt.expected)
			}

			// 验证往返转换
			if len(tt.input) > 0 {
				// 转换回 float32 并验证
				back := bytesToFloat32(result)
				for i := range tt.input {
					// 允许一定的精度损失
					if abs(back[i]-tt.input[i]) > 0.0001 {
						t.Errorf("往返转换样本 %d 误差过大: got %f, want %f", i, back[i], tt.input[i])
					}
				}
			}
		})
	}
}

func TestBytesToFloat32(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int // 期望的 float32 长度
	}{
		{
			name:     "转换字节数组",
			input:    []byte{0x00, 0x80, 0x00, 0x80}, // 两个 int16 样本
			expected: 2,
		},
		{
			name:     "空输入",
			input:    []byte{},
			expected: 0,
		},
		{
			name:     "单样本",
			input:    []byte{0xFF, 0x7F}, // 最大正值
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bytesToFloat32(tt.input)

			if len(result) != tt.expected {
				t.Errorf("float32 长度错误: got %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "连接重置",
			errMsg:   "connection reset by peer",
			expected: true,
		},
		{
			name:     "管道断开",
			errMsg:   "broken pipe",
			expected: true,
		},
		{
			name:     "普通错误",
			errMsg:   "some other error",
			expected: false,
		},
		{
			name:     "空错误",
			errMsg:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNetworkError(&testError{msg: tt.errMsg})
			if result != tt.expected {
				t.Errorf("isNetworkError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// 辅助函数和类型
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// bytesToFloat32 将 int16 PCM 字节转换为 float32
func bytesToFloat32(data []byte) []float32 {
	numSamples := len(data) / 2
	if numSamples == 0 {
		return nil
	}
	samples := make([]float32, numSamples)

	for i := 0; i < numSamples; i++ {
		sample := int16(data[i*2]) | int16(data[i*2+1])<<8
		samples[i] = float32(sample) / 32768.0
	}

	return samples
}
