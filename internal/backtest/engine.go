package backtest

import "math"

// RunBacktest 执行回测。
// 纯函数：输入策略 + K线数据 + 初始资金 → 输出回测报告。
// commission: 单边手续费比例（默认 0.0003，即万三）。
// slippage: 滑点比例（默认 0）。
func RunBacktest(strategy Strategy, bars []Bar, initialCapital float64, commission float64) *BacktestReport {
	if len(bars) < 2 || initialCapital <= 0 {
		return &BacktestReport{}
	}
	if commission < 0 {
		commission = 0.0003
	}

	port := Portfolio{Cash: initialCapital}
	var trades []Trade
	var dailyEquity []float64    // 每日资产（用于回撤/夏普计算）
	var equityCurve []Compound   // 带日期的资产曲线

	dailyEquity = append(dailyEquity, initialCapital)
	equityCurve = append(equityCurve, Compound{Time: bars[0].Time, Equity: initialCapital})

	for i := 1; i < len(bars); i++ {
		ctx := BacktestContext{
			Index:     i,
			Bar:       bars[i],
			History:   bars[:i+1],
			Portfolio: port,
		}
		signal := strategy.Evaluate(ctx)

		switch signal.Action {
		case Buy:
			if port.Cash <= 0 {
				break
			}
			investAmount := port.Cash * clamp(signal.Quantity, 0.01, 1.0)
			price := bars[i].Close
			qty := math.Floor(investAmount / price / 100) * 100 // A股100股整数倍
			if qty <= 0 {
				break
			}
			cost := qty*price + qty*price*commission
			if cost > port.Cash {
				cost = port.Cash
				qty = math.Floor(cost / (price * (1 + commission)) / 100) * 100
				if qty <= 0 {
					break
				}
				cost = qty*price + qty*price*commission
			}
			port.Cash -= cost
			// 加权平均成本
			oldTotal := port.Position * port.AvgCost
			port.AvgCost = (oldTotal + cost) / (port.Position + qty)
			port.Position += qty
			trades = append(trades, Trade{
				Time: bars[i].Time, Action: Buy, Price: price,
				Quantity: qty, Cost: cost, Reason: signal.Reason,
			})

		case Sell:
			if port.Position <= 0 {
				break
			}
			sellQty := port.Position * clamp(signal.Quantity, 0.01, 1.0)
			sellQty = math.Floor(sellQty/100) * 100
			if sellQty <= 0 {
				sellQty = port.Position
			}
			if sellQty > port.Position {
				sellQty = port.Position
			}
			price := bars[i].Close
			proceed := sellQty*price - sellQty*price*commission
			port.Cash += proceed
			port.Position -= sellQty
			if port.Position == 0 {
				port.AvgCost = 0
			}
			trades = append(trades, Trade{
				Time: bars[i].Time, Action: Sell, Price: price,
				Quantity: sellQty, Cost: proceed, Reason: signal.Reason,
			})
		}

		equity := port.Cash + port.Position*bars[i].Close
		dailyEquity = append(dailyEquity, equity)
		equityCurve = append(equityCurve, Compound{Time: bars[i].Time, Equity: equity})
	}

	finalEquity := dailyEquity[len(dailyEquity)-1]
	totalReturn := (finalEquity - initialCapital) / initialCapital * 100

	// 计算年化收益率（假设250个交易日/年，按实际天数线性缩放）
	tradingDays := len(bars) - 1
	if tradingDays > 0 {
		annualFactor := float64(tradingDays) / 250.0
		if annualFactor > 0 {
			annualReturn := (math.Pow(finalEquity/initialCapital, 1.0/annualFactor) - 1) * 100
			report := buildReport(initialCapital, finalEquity, totalReturn, dailyEquity, trades, commission, bars, annualReturn, tradingDays)
			report.EquityCurve = equityCurve
			return report
		}
	}

	report := buildReport(initialCapital, finalEquity, totalReturn, dailyEquity, trades, commission, bars, 0, tradingDays)
	report.EquityCurve = equityCurve
	return report
}

func buildReport(initial, final, totalReturn float64, dailyEquity []float64, trades []Trade, commission float64, bars []Bar, annualReturn float64, tradingDays int) *BacktestReport {
	drawdown := maxDrawdown(dailyEquity)
	sharpe := sharpeRatio(dailyEquity, 0.03) // 无风险利率 3%
	winRate, wins := winRateCalc(trades)
	totalCommission := sumCommission(trades, commission)

	// 年化波动率
	dailyReturns := make([]float64, len(dailyEquity)-1)
	for i := 1; i < len(dailyEquity); i++ {
		dailyReturns[i-1] = (dailyEquity[i] - dailyEquity[i-1]) / dailyEquity[i-1]
	}
	annVol := annualVolatility(dailyReturns)

	return &BacktestReport{
		TotalReturn:      totalReturn,
		AnnualReturn:     annualReturn,
		MaxDrawdown:      drawdown,
		SharpeRatio:      sharpe,
		WinRate:          winRate,
		TotalTrades:      len(trades),
		WinningTrades:    wins,
		InitialCapital:   initial,
		FinalEquity:      final,
		Trades:           trimTrades(trades),
		Commission:       totalCommission,
		AnnualVolatility: annVol,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}

func trimTrades(trades []Trade) []Trade {
	if len(trades) <= 100 {
		return trades
	}
	return trades[len(trades)-100:]
}
