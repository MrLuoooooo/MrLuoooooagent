package indicator

// OBV 能量潮。
// 今日收盘 > 昨日收盘 → +成交量
// 今日收盘 < 昨日收盘 → -成交量
// 今日收盘 = 昨日收盘 → 0
// 纯函数，无副作用。prices 和 volumes 长度须一致。
func OBV(prices, volumes []float64) []float64 {
	n := len(prices)
	if n <= 1 || len(volumes) != n {
		return nil
	}
	obv := make([]float64, n)
	obv[0] = volumes[0]
	for i := 1; i < n; i++ {
		if prices[i] > prices[i-1] {
			obv[i] = obv[i-1] + volumes[i]
		} else if prices[i] < prices[i-1] {
			obv[i] = obv[i-1] - volumes[i]
		} else {
			obv[i] = obv[i-1]
		}
	}
	return obv
}
