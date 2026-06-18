package backtest

import "math"

// maxDrawdown 计算最大回撤(%)。
// 纯函数。
func maxDrawdown(equity []float64) float64 {
	if len(equity) < 2 {
		return 0
	}
	peak := equity[0]
	maxDD := 0.0
	for _, e := range equity {
		if e > peak {
			peak = e
		}
		dd := (peak - e) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// sharpeRatio 计算夏普比率（年化）。
// riskFree 无风险年化利率（如 0.03）。
// 纯函数。
func sharpeRatio(dailyEquity []float64, riskFree float64) float64 {
	if len(dailyEquity) < 2 {
		return 0
	}
	dailyReturns := make([]float64, len(dailyEquity)-1)
	for i := 1; i < len(dailyEquity); i++ {
		dailyReturns[i-1] = (dailyEquity[i] - dailyEquity[i-1]) / dailyEquity[i-1]
	}
	mean := average(dailyReturns)
	std := stdDev(dailyReturns, mean)
	if std == 0 {
		return 0
	}
	// 年化：日均 * sqrt(250)
	annualMean := mean * 250
	annualStd := std * math.Sqrt(250)
	annualRiskFree := riskFree // already annual
	return (annualMean - annualRiskFree) / annualStd
}

// winRateCalc 计算胜率(%)。
// 买入后下一笔卖出为一次完整交易，比较盈亏。
func winRateCalc(trades []Trade) (float64, int) {
	if len(trades) < 2 {
		return 0, 0
	}
	completed := 0
	wins := 0
	var buyTrade *Trade
	for i := range trades {
		t := &trades[i]
		if t.Action == Buy {
			buyTrade = t
		} else if t.Action == Sell && buyTrade != nil {
			completed++
			if t.Price > buyTrade.Price {
				wins++
			}
			buyTrade = nil
		}
	}
	if completed == 0 {
		return 0, 0
	}
	return float64(wins) / float64(completed) * 100, wins
}

// sumCommission 计算总手续费。
// trades[i].Cost 在 engine.go 中已含手续费，此处不应再次应用费率。
// 对于买入：Cost = qty*price + qty*price*commission，手续费部分 = qty*price*commission
// 对于卖出：Cost = sellQty*price - sellQty*price*commission（净收入），手续费部分 = sellQty*price*commission
// 简化：Trade.Cost 字段存储的是总成本/净收入，手续费 = |Cost - qty*price|
// 此处用简单估算：每笔交易按成交额的 rate 计算。
func sumCommission(trades []Trade, rate float64) float64 {
	var total float64
	for _, t := range trades {
		// Cost 字段含义不同（买入=含手续费成本，卖出=净收入），
		// 统一按成交额 * 费率计算手续费。
		total += t.Price * float64(t.Quantity) * rate
	}
	return total
}

// annualVolatility 计算年化波动率(%)。
func annualVolatility(dailyReturns []float64, tradingDays float64) float64 {
	if len(dailyReturns) == 0 || tradingDays == 0 {
		return 0
	}
	mean := average(dailyReturns)
	std := stdDev(dailyReturns, mean)
	return std * math.Sqrt(250) * 100
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stdDev(vals []float64, mean float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, v := range vals {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(vals)-1))
}
