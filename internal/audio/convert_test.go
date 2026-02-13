package audio

import (
	"math"
	"testing"
)

func TestInt16ToFloat32_Empty(t *testing.T) {
	out := Int16ToFloat32(nil)
	if len(out) != 0 {
		t.Fatalf("expected empty slice, got length %d", len(out))
	}
}

func TestInt16ToFloat32_Zero(t *testing.T) {
	out := Int16ToFloat32([]int16{0})
	if out[0] != 0 {
		t.Fatalf("expected 0.0, got %f", out[0])
	}
}

func TestInt16ToFloat32_MaxInt16(t *testing.T) {
	out := Int16ToFloat32([]int16{math.MaxInt16})
	if out[0] != 1.0 {
		t.Fatalf("expected 1.0 for MaxInt16, got %f", out[0])
	}
}

func TestInt16ToFloat32_MinInt16(t *testing.T) {
	out := Int16ToFloat32([]int16{math.MinInt16})
	// MinInt16 = -32768, MaxInt16 = 32767, so result ≈ -1.000030518
	expected := float32(math.MinInt16) / math.MaxInt16
	if out[0] != expected {
		t.Fatalf("expected %f for MinInt16, got %f", expected, out[0])
	}
}

func TestFloat32ToInt16_Normal(t *testing.T) {
	out := Float32ToInt16([]float32{0.5, -0.5, 0})
	if out[2] != 0 {
		t.Fatalf("expected 0 for 0.0 input, got %d", out[2])
	}
	if out[0] <= 0 {
		t.Fatalf("expected positive for 0.5 input, got %d", out[0])
	}
	if out[1] >= 0 {
		t.Fatalf("expected negative for -0.5 input, got %d", out[1])
	}
}

func TestFloat32ToInt16_ClampHigh(t *testing.T) {
	out := Float32ToInt16([]float32{1.5})
	expected := int16(1.0 * math.MaxInt16)
	if out[0] != expected {
		t.Fatalf("expected %d (clamped to 1.0), got %d", expected, out[0])
	}
}

func TestFloat32ToInt16_ClampLow(t *testing.T) {
	out := Float32ToInt16([]float32{-1.5})
	expected := int16(-1.0 * math.MaxInt16)
	if out[0] != expected {
		t.Fatalf("expected %d (clamped to -1.0), got %d", expected, out[0])
	}
}

func TestBytesToInt16_LittleEndian(t *testing.T) {
	// 0x0102 in little-endian is {0x02, 0x01}
	b := []byte{0x02, 0x01}
	out := BytesToInt16(b)
	if len(out) != 1 || out[0] != 0x0102 {
		t.Fatalf("expected 258 (0x0102), got %d", out[0])
	}
}

func TestInt16ToBytes_LittleEndian(t *testing.T) {
	out := Int16ToBytes([]int16{0x0102})
	if len(out) != 2 || out[0] != 0x02 || out[1] != 0x01 {
		t.Fatalf("expected [0x02, 0x01], got %v", out)
	}
}

func TestBytesInt16_Roundtrip(t *testing.T) {
	samples := []int16{0, 1, -1, 1000, -1000, math.MaxInt16, math.MinInt16}
	b := Int16ToBytes(samples)
	result := BytesToInt16(b)
	if len(result) != len(samples) {
		t.Fatalf("length mismatch: expected %d, got %d", len(samples), len(result))
	}
	for i, s := range samples {
		if result[i] != s {
			t.Errorf("index %d: expected %d, got %d", i, s, result[i])
		}
	}
}

func TestBytesFloat32_Roundtrip(t *testing.T) {
	// Note: roundtrip through int16 introduces quantization,
	// so we use values that survive the conversion cleanly.
	input := []float32{0, 1.0, -1.0}
	b := Float32ToBytes(input)
	output := BytesToFloat32(b)
	if len(output) != len(input) {
		t.Fatalf("length mismatch: expected %d, got %d", len(input), len(output))
	}
	// 0.0 should roundtrip exactly
	if output[0] != 0 {
		t.Errorf("expected 0.0, got %f", output[0])
	}
	// 1.0 should roundtrip exactly (MaxInt16 / MaxInt16 = 1.0)
	if output[1] != 1.0 {
		t.Errorf("expected 1.0, got %f", output[1])
	}
}
