package indicator

// RSI 相对强弱指标。
// 使用 Wilder's smoothing：首次用简单平均，后续 EMA 递推。
// 纯函数，无副作用。
func RSI(prices []float64, period int) []float64 {
	n := len(prices)
	if n <= 1 || period <= 0 || n <= period {
		return make([]float64, n)
	}

	gains := make([]float64, n)
	losses := make([]float64, n)
	for i := 1; i < n; i++ {
		delta := prices[i] - prices[i-1]
		if delta > 0 {
			gains[i] = delta
		} else {
			losses[i] = -delta
		}
	}

	// 首次 average gain/loss = 简单平均。
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	result := make([]float64, n)
	if avgLoss == 0 {
		result[period] = 100
	} else {
		rs := avgGain / avgLoss
		result[period] = 100 - 100/(1+rs)
	}

	// 后续用 EMA 平滑。
	for i := period + 1; i < n; i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = 100 - 100/(1+rs)
		}
	}
	return result
}
