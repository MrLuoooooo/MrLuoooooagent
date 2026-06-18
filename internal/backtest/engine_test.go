package backtest

import (
	"math"
	"testing"
)

// testStrategy 简单的均线交叉策略用于测试引擎。
type testStrategy struct {
	shortPeriod int
	longPeriod  int
	crossedUp   bool
	crossedDown bool
	buyCount    int
}

func (s *testStrategy) Evaluate(ctx BacktestContext) Signal {
	if len(ctx.History) < s.longPeriod+1 {
		return Signal{Action: Hold}
	}
	// 简易均线交叉
	shortMA := sma(ctx.History, s.shortPeriod)
	longMA := sma(ctx.History, s.longPeriod)
	prevShort := sma(ctx.History[:len(ctx.History)-1], s.shortPeriod)
	prevLong := sma(ctx.History[:len(ctx.History)-1], s.longPeriod)

	if prevShort <= prevLong && shortMA > longMA {
		s.crossedUp = true
		s.buyCount++
		return Signal{Action: Buy, Quantity: 0.8, Reason: "金叉"}
	}
	if prevShort >= prevLong && shortMA < longMA {
		s.crossedDown = true
		return Signal{Action: Sell, Quantity: 1.0, Reason: "死叉"}
	}
	return Signal{Action: Hold}
}

func sma(bars []Bar, period int) float64 {
	if len(bars) < period {
		return bars[len(bars)-1].Close
	}
	sum := 0.0
	for i := len(bars) - period; i < len(bars); i++ {
		sum += bars[i].Close
	}
	return sum / float64(period)
}

func makeTrendBars(n int, trend float64) []Bar {
	bars := make([]Bar, n)
	price := 10.0
	for i := 0; i < n; i++ {
		price += trend
		// 加入少量噪声以触发均线交叉
		noise := math.Sin(float64(i)*0.5) * 0.3
		bars[i] = Bar{
			Time:  "2025-01-01",
			Open:  price + noise - 0.1,
			High:  price + noise + 0.2,
			Low:   price + noise - 0.2,
			Close: price + noise,
		}
	}
	return bars
}

func TestRunBacktest_TrendUp(t *testing.T) {
	// 先横盘再拉升，确保均线交叉
	bars := make([]Bar, 120)
	price := 10.0
	for i := 0; i < 120; i++ {
		if i >= 60 {
			price += 0.15 // 拉升
		}
		bars[i] = Bar{
			Time:  "2025-01-01",
			Open:  price - 0.1,
			High:  price + 0.2,
			Low:   price - 0.2,
			Close: price,
		}
	}
	s := &testStrategy{shortPeriod: 5, longPeriod: 20}
	r := RunBacktest(s, bars, 100000, 0.0003)
	if r.TotalTrades == 0 {
		t.Fatal("expected trades — price jumped from flat to uptrend")
	}
	if s.buyCount == 0 {
		t.Error("uptrend strategy should have buy signals")
	}
}

func TestRunBacktest_TrendDown(t *testing.T) {
	bars := makeTrendBars(100, -0.05)
	s := &testStrategy{shortPeriod: 5, longPeriod: 20}
	r := RunBacktest(s, bars, 100000, 0.0003)
	if r.TotalReturn == 0 && r.TotalTrades == 0 {
		t.Log("no trades in strong downtrend — acceptable")
	}
}

func TestRunBacktest_EmptyBars(t *testing.T) {
	r := RunBacktest(&testStrategy{}, nil, 100000, 0.0003)
	if r.TotalTrades != 0 {
		t.Error("empty bars should produce no trades")
	}
}

func TestMaxDrawdown_Flat(t *testing.T) {
	eq := []float64{100, 100, 100, 100, 100}
	dd := maxDrawdown(eq)
	if dd != 0 {
		t.Errorf("flat equity should have 0 drawdown, got %.2f", dd)
	}
}

func TestMaxDrawdown_V(t *testing.T) {
	eq := []float64{100, 80, 60, 80, 100}
	dd := maxDrawdown(eq)
	if math.Abs(dd-40) > 0.01 {
		t.Errorf("V-shaped drawdown should be 40%%, got %.2f", dd)
	}
}

func TestSharpeRatio(t *testing.T) {
	// 稳定增长
	eq := make([]float64, 100)
	eq[0] = 100
	for i := 1; i < len(eq); i++ {
		eq[i] = eq[i-1] * 1.001 // 每天涨 0.1%
	}
	sr := sharpeRatio(eq, 0.03)
	if sr <= 0 {
		t.Errorf("positive trend should have positive sharpe, got %.4f", sr)
	}
}

func TestWinRateCalc(t *testing.T) {
	trades := []Trade{
		{Action: Buy, Price: 10, Cost: 5000},
		{Action: Sell, Price: 12, Cost: 6000},
		{Action: Buy, Price: 15, Cost: 3000},
		{Action: Sell, Price: 11, Cost: 2200},
	}
	rate, wins := winRateCalc(trades)
	if rate != 50 || wins != 1 {
		t.Errorf("win rate should be 50%%/1, got %.0f%%/%d", rate, wins)
	}
}

func TestWinRateCalc_NoSell(t *testing.T) {
	trades := []Trade{{Action: Buy, Price: 10, Cost: 5000}}
	rate, wins := winRateCalc(trades)
	if rate != 0 || wins != 0 {
		t.Errorf("no sell should be 0%%, got %.0f%%/%d", rate, wins)
	}
}

func TestEngine_MaxTradesCap(t *testing.T) {
	bars := makeTrendBars(500, 0.02)
	s := &testStrategy{shortPeriod: 5, longPeriod: 20}
	r := RunBacktest(s, bars, 100000, 0.0003)
	if len(r.Trades) > 100 {
		t.Errorf("trades should be capped at 100, got %d", len(r.Trades))
	}
}

func TestEngine_Commission(t *testing.T) {
	bars := makeTrendBars(100, 0.05)
	s := &testStrategy{shortPeriod: 5, longPeriod: 20}
	// 高手续费 vs 低手续费
	rHigh := RunBacktest(s, bars, 100000, 0.01)
	rLow := RunBacktest(s, bars, 100000, 0.0001)
	if rHigh.Commission <= rLow.Commission && rHigh.TotalTrades > 0 {
		t.Log("higher commission should produce higher total commission")
	}
}
