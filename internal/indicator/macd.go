package indicator

// MACD 计算结果。
type MACDResult struct {
	DIFF []float64 // 快线 (EMA12 - EMA26)
	DEA  []float64 // 慢线 (EMA9 of DIFF)
	MACD []float64 // 柱状线 2*(DIFF - DEA)
}

// MACD 指数平滑异同移动平均线。
// 标准参数：fast=12, slow=26, signal=9。
// 纯函数，无副作用。
func MACD(prices []float64, fast, slow, signal int) *MACDResult {
	n := len(prices)
	maxPeriod := fast
	if slow > maxPeriod { maxPeriod = slow }
	if signal > maxPeriod { maxPeriod = signal }
	nilResult := func() *MACDResult {
		z := make([]float64, n)
		return &MACDResult{DIFF: z, DEA: z, MACD: z}
	}
	if n == 0 || fast <= 0 || slow <= 0 || signal <= 0 || n <= maxPeriod {
		return nilResult()
	}
	emaFast := EMA(prices, fast)
	emaSlow := EMA(prices, slow)
	maxIdx := maxPeriodIdx(n, slow)

	diff := make([]float64, n)
	for i := 0; i < n; i++ {
		if i >= maxIdx {
			diff[i] = emaFast[i] - emaSlow[i]
		}
	}

	// slow-1 可能 >= n，加保护
	deaStart := slow - 1
	if deaStart >= n {
		return nilResult()
	}
	dea := EMA(diff[deaStart:], signal)
	deaPadded := make([]float64, n)
	copyStart := deaStart + signal - 1
	if copyStart < 0 {
		copyStart = 0
	}
	if copyStart < n {
		copy(deaPadded[copyStart:], dea)
	}

	macdBar := make([]float64, n)
	for i := 0; i < n; i++ {
		macdBar[i] = 2 * (diff[i] - deaPadded[i])
	}

	return &MACDResult{DIFF: diff, DEA: deaPadded, MACD: macdBar}
}

func maxPeriodIdx(n, period int) int {
	if n < period {
		return n
	}
	return period - 1
}
