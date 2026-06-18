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
	if len(prices) == 0 || fast <= 0 || slow <= 0 || signal <= 0 {
		return &MACDResult{}
	}
	emaFast := EMA(prices, fast)
	emaSlow := EMA(prices, slow)
	maxIdx := maxPeriodIdx(len(prices), slow)

	diff := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		if i >= maxIdx {
			diff[i] = emaFast[i] - emaSlow[i]
		}
	}

	dea := EMA(diff[slow-1:], signal)
	deaPadded := make([]float64, len(prices))
	copyStart := slow - 1 + signal - 1
	if copyStart < 0 {
		copyStart = 0
	}
	copy(deaPadded[copyStart:], dea)

	macdBar := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
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
