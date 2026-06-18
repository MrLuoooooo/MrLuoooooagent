package indicator

// SMA 简单移动平均。
// 纯函数：输入 prices + period → 输出 SMA 序列，无副作用。
// 长度不足 period 时前 period-1 个位置返回 0。
func SMA(prices []float64, period int) []float64 {
	n := len(prices)
	if n == 0 || period <= 0 {
		return nil
	}
	result := make([]float64, n)
	if n < period {
		return result
	}
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	result[period-1] = sum / float64(period)
	for i := period; i < n; i++ {
		sum += prices[i] - prices[i-period]
		result[i] = sum / float64(period)
	}
	return result
}

// EMA 指数移动平均。
// alpha = 2/(period+1)，第一个有效值为 SMA(period)，之后递归。
func EMA(prices []float64, period int) []float64 {
	n := len(prices)
	if n == 0 || period <= 0 {
		return nil
	}
	result := make([]float64, n)
	if n < period {
		return result
	}
	alpha := 2.0 / float64(period+1)
	// 用 SMA(period) 作为初始种子。
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	result[period-1] = sum / float64(period)
	for i := period; i < n; i++ {
		result[i] = alpha*prices[i] + (1-alpha)*result[i-1]
	}
	return result
}

// WMA 加权移动平均。
// 最近一天权重=period，最远一天权重=1，分母=period*(period+1)/2。
func WMA(prices []float64, period int) []float64 {
	n := len(prices)
	if n == 0 || period <= 0 {
		return nil
	}
	result := make([]float64, n)
	if n < period {
		return result
	}
	denom := float64(period*(period+1)) / 2.0
	for i := period - 1; i < n; i++ {
		var sum float64
		for j := 0; j < period; j++ {
			sum += prices[i-j] * float64(period-j)
		}
		result[i] = sum / denom
	}
	return result
}
