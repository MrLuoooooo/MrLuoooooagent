package indicator

import (
	"math"
	"testing"
)

func TestSMA(t *testing.T) {
	prices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	r := SMA(prices, 3)
	// SMA(3): [0 0 2 3 4 5 6 7 8 9]
	expected := []float64{0, 0, 2, 3, 4, 5, 6, 7, 8, 9}
	for i, v := range expected {
		if math.Abs(r[i]-v) > 0.001 {
			t.Fatalf("SMA[%d]: want %.4f, got %.4f", i, v, r[i])
		}
	}
}

func TestEMA(t *testing.T) {
	prices := []float64{10, 10, 10, 10, 10, 20, 20, 20, 20, 20}
	r := EMA(prices, 5)
	// EMA(5) seed=SMA(1..5)=10 at index 4, then alpha=2/6=0.333
	// idx 4: 10
	// idx 5: 0.333*20 + 0.667*10 = 13.333
	// idx 6: 0.333*20 + 0.667*13.333 = 15.556
	// idx 7: 0.333*20 + 0.667*15.556 = 17.037
	// idx 8: 0.333*20 + 0.667*17.037 = 18.025
	// idx 9: 0.333*20 + 0.667*18.025 = 18.683
	epsilon := 0.3
	if math.Abs(r[4]-10) > epsilon ||
		math.Abs(r[5]-13.333) > epsilon ||
		math.Abs(r[6]-15.556) > epsilon {
		t.Fatalf("EMA: unexpected values, got %v", tailN(r, 6))
	}
}

func TestMACD(t *testing.T) {
	// Generate 50 data points.
	prices := make([]float64, 50)
	for i := range prices {
		prices[i] = 10 + float64(i)*0.1 + math.Sin(float64(i)*0.3)*2
	}
	r := MACD(prices, 12, 26, 9)
	if len(r.DIFF) != 50 || len(r.DEA) != 50 || len(r.MACD) != 50 {
		t.Fatal("MACD result length mismatch")
	}
	// DIFF and DEA should be non-zero after period 25.
	allZero := true
	for i := 30; i < 50; i++ {
		if math.Abs(r.DIFF[i]) > 0.0001 || math.Abs(r.DEA[i]) > 0.0001 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("MACD DIFF/DEA should be non-zero for trending data")
	}
}

func TestRSI(t *testing.T) {
	// All gains.
	prices := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	r := RSI(prices, 14)
	// Index 14 should be 100 (all gains).
	if math.Abs(r[14]-100) > 0.01 {
		t.Fatalf("RSI all gains: want 100, got %.4f", r[14])
	}
}

func TestBOLL(t *testing.T) {
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = 10 + float64(i)*0.1
	}
	r := BOLL(prices, 20, 2)
	// After 19, middle/upper/lower should be non-zero.
	if r.Middle[29] == 0 || r.Upper[29] == 0 || r.Lower[29] == 0 {
		t.Fatal("BOLL bands should be non-zero")
	}
	if r.Upper[29] < r.Middle[29] || r.Lower[29] > r.Middle[29] {
		t.Fatal("BOLL: upper > middle > lower")
	}
}

func TestKDJ(t *testing.T) {
	n := 30
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		close[i] = 20 + float64(i)*0.2
		high[i] = close[i] + 1
		low[i] = close[i] - 1
	}
	r := KDJ(high, low, close, 9)
	if len(r.K) != n || len(r.D) != n || len(r.J) != n {
		t.Fatal("KDJ result length mismatch")
	}
	if r.K[n-1] == 0 {
		t.Fatal("KDJ K value should be non-zero")
	}
}

func TestATR(t *testing.T) {
	n := 20
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	for i := 0; i < n; i++ {
		close[i] = 10 + float64(i)*0.1
		high[i] = close[i] + 0.5
		low[i] = close[i] - 0.5
	}
	r := ATR(high, low, close, 14)
	if r[n-1] < 0.8 || r[n-1] > 1.2 {
		t.Fatalf("ATR should be ~1.0, got %.4f", r[n-1])
	}
}

func TestOBV(t *testing.T) {
	prices := []float64{10, 11, 10.5, 12, 11}
	volumes := []float64{100, 150, 200, 100, 80}
	r := OBV(prices, volumes)
	// Day 0: 100
	// Day 1: up, 100+150=250
	// Day 2: down, 250-200=50
	// Day 3: up, 50+100=150
	// Day 4: down, 150-80=70
	expected := []float64{100, 250, 50, 150, 70}
	for i, v := range expected {
		if math.Abs(r[i]-v) > 0.01 {
			t.Fatalf("OBV[%d]: want %.4f, got %.4f", i, v, r[i])
		}
	}
}

func TestEmptyInput(t *testing.T) {
	if SMA(nil, 5) != nil {
		t.Fatal("SMA nil should return nil")
	}
	if len(SMA([]float64{}, 5)) != 0 {
		t.Fatal("SMA empty should return empty")
	}
	if MACD(nil, 12, 26, 9) == nil {
		t.Fatal("MACD nil should return empty struct")
	}
	if r := RSI([]float64{1, 2}, 14); len(r) != 2 {
		t.Fatal("RSI short data should return same length")
	}
	if r := KDJ(nil, nil, nil, 9); len(r.K) != 0 {
		t.Fatal("KDJ nil should return empty")
	}
	if r := ATR(nil, nil, nil, 14); len(r) != 0 {
		t.Fatal("ATR nil should return empty")
	}
	if OBV(nil, nil) != nil {
		t.Fatal("OBV nil should return nil")
	}
	if OBV([]float64{1, 2}, []float64{1}) != nil {
		t.Fatal("OBV mismatched lengths should return nil")
	}
}

func tailN(r []float64, n int) []float64 {
	if len(r) <= n {
		return r
	}
	return r[len(r)-n:]
}
