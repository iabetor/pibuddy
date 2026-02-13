package audio

import (
	"math"
)

// Int16ToFloat32 将 PCM int16 样本转换为 [-1.0, 1.0] 范围的 float32。
func Int16ToFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, s := range in {
		out[i] = float32(s) / math.MaxInt16
	}
	return out
}

// Float32ToInt16 将 [-1.0, 1.0] 范围的 float32 样本转换为 PCM int16。
func Float32ToInt16(in []float32) []int16 {
	out := make([]int16, len(in))
	for i, s := range in {
		// 钳位到 [-1.0, 1.0]
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		out[i] = int16(s * math.MaxInt16)
	}
	return out
}

// BytesToInt16 将小端字节切片转换为 int16 样本。
func BytesToInt16(b []byte) []int16 {
	n := len(b) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(b[2*i]) | int16(b[2*i+1])<<8
	}
	return out
}

// Int16ToBytes 将 int16 样本转换为小端字节切片。
func Int16ToBytes(in []int16) []byte {
	out := make([]byte, len(in)*2)
	for i, s := range in {
		out[2*i] = byte(s)
		out[2*i+1] = byte(s >> 8)
	}
	return out
}

// BytesToFloat32 便捷函数：将原始 PCM 字节直接转换为 float32。
func BytesToFloat32(b []byte) []float32 {
	return Int16ToFloat32(BytesToInt16(b))
}

// Float32ToBytes 便捷函数：将 float32 样本直接转换为原始 PCM 字节。
func Float32ToBytes(in []float32) []byte {
	return Int16ToBytes(Float32ToInt16(in))
}
