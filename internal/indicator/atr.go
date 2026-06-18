package indicator

import "math"

// ATR 平均真实波幅。
// True Range = max(High-Low, abs(High-prevClose), abs(Low-prevClose))
// 首次 ATR = SMA(TR, period)，后续 EMA 递推。
// 纯函数，无副作用。
func ATR(high, low, close []float64, period int) []float64 {
	n := len(close)
	if n <= 1 || period <= 0 || n <= period {
		return make([]float64, n)
	}

	tr := make([]float64, n)
	for i := 1; i < n; i++ {
		hl := high[i] - low[i]
		hpc := math.Abs(high[i] - close[i-1])
		lpc := math.Abs(low[i] - close[i-1])
		tr[i] = math.Max(hl, math.Max(hpc, lpc))
	}

	// 首次 ATR = SMA(TR, period)。
	var sum float64
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	atr := make([]float64, n)
	atr[period] = sum / float64(period)

	// 后续 EMA。
	for i := period + 1; i < n; i++ {
		atr[i] = (atr[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return atr
}
