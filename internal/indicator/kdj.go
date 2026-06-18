package indicator

// KDJResult KDJ 三条线。
type KDJResult struct {
	K []float64
	D []float64
	J []float64
}

// KDJ 随机指标。
// RSV = (Close - Lowest(n)) / (Highest(n) - Lowest(n)) * 100
// K = 2/3 * prevK + 1/3 * RSV
// D = 2/3 * prevD + 1/3 * K
// J = 3*K - 2*D
// 标准 period=9。
// 纯函数，无副作用。
func KDJ(high, low, close []float64, period int) *KDJResult {
	n := len(close)
	if n == 0 || period <= 0 || n < period {
		return &KDJResult{}
	}
	k := make([]float64, n)
	d := make([]float64, n)
	j := make([]float64, n)

	// 初始 K=50, D=50。
	prevK, prevD := 50.0, 50.0
	for i := period - 1; i < n; i++ {
		highest := high[i]
		lowest := low[i]
		for m := i - period + 1; m <= i; m++ {
			if high[m] > highest {
				highest = high[m]
			}
			if low[m] < lowest {
				lowest = low[m]
			}
		}
		var rsv float64
		if highest == lowest {
			rsv = 50
		} else {
			rsv = (close[i] - lowest) / (highest - lowest) * 100
		}
		k[i] = prevK*2/3 + rsv/3
		d[i] = prevD*2/3 + k[i]/3
		j[i] = 3*k[i] - 2*d[i]
		prevK = k[i]
		prevD = d[i]
	}

	return &KDJResult{K: k, D: d, J: j}
}
