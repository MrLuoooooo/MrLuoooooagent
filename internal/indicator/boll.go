package indicator

import "math"

// BOLLResult 布林带三条线。
type BOLLResult struct {
	Upper  []float64 // 上轨 = MIDDLE + multiplier * stdDev
	Middle []float64 // 中轨 = SMA(period)
	Lower  []float64 // 下轨 = MIDDLE - multiplier * stdDev
}

// BOLL 布林带。
// 标准参数：period=20, multiplier=2。
// 纯函数，无副作用。
func BOLL(prices []float64, period int, multiplier float64) *BOLLResult {
	n := len(prices)
	if n == 0 || period <= 0 || multiplier <= 0 {
		return &BOLLResult{Upper: []float64{}, Middle: []float64{}, Lower: []float64{}}
	}

	middle := SMA(prices, period)
	upper := make([]float64, n)
	lower := make([]float64, n)

	for i := period - 1; i < n; i++ {
		// 计算窗口标准差。
		slice := prices[i-period+1 : i+1]
		mean := middle[i]
		var variance float64
		for _, v := range slice {
			diff := v - mean
			variance += diff * diff
		}
		variance /= float64(period)
		stdDev := math.Sqrt(variance)

		upper[i] = mean + multiplier*stdDev
		lower[i] = mean - multiplier*stdDev
	}

	return &BOLLResult{Upper: upper, Middle: middle, Lower: lower}
}
