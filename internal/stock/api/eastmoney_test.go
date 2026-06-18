package api

import (
	"testing"
)

func TestParseFinancial(t *testing.T) {
	// 模拟东方财富 push2 API 响应（精简字段）
	jsonData := []byte(`{
		"data": {
			"f58": "贵州茅台",
			"f37": 30.5,
			"f38": 150.2,
			"f39": 12.3,
			"f40": 150000000000,
			"f41": 75000000000,
			"f42": 59.68,
			"f46": 15.3,
			"f47": 12.8,
			"f48": 92.5,
			"f49": 50.0,
			"f115": 25.5,
			"f117": 8.3
		}
	}`)

	client := &EastMoneyClient{}
	d, err := client.parseFinancial("sh600519", jsonData)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.Code != "sh600519" {
		t.Errorf("code: got %s, want sh600519", d.Code)
	}
	if d.Name != "贵州茅台" {
		t.Errorf("name: got %s, want 贵州茅台", d.Name)
	}
	if d.PE != 25.5 {
		t.Errorf("PE: got %.1f, want 25.5", d.PE)
	}
	if d.PB != 8.3 {
		t.Errorf("PB: got %.1f, want 8.3", d.PB)
	}
	if d.ROE != 30.5 {
		t.Errorf("ROE: got %.1f, want 30.5", d.ROE)
	}
	if d.EPS != 59.68 {
		t.Errorf("EPS: got %.1f, want 59.68", d.EPS)
	}
	if d.BPS != 150.2 {
		t.Errorf("BPS: got %.1f, want 150.2", d.BPS)
	}
	if d.CFPS != 12.3 {
		t.Errorf("CFPS: got %.1f, want 12.3", d.CFPS)
	}
	// 营收 150000000000 元 → 1500 亿
	if d.Revenue != 1500 {
		t.Errorf("Revenue: got %.0f, want 1500 (亿)", d.Revenue)
	}
	// 净利 75000000000 元 → 750 亿
	if d.NetProfit != 750 {
		t.Errorf("NetProfit: got %.0f, want 750 (亿)", d.NetProfit)
	}
	if d.RevenueYoY != 15.3 {
		t.Errorf("RevenueYoY: got %.1f, want 15.3", d.RevenueYoY)
	}
	if d.ProfitYoY != 12.8 {
		t.Errorf("ProfitYoY: got %.1f, want 12.8", d.ProfitYoY)
	}
	if d.GrossMargin != 92.5 {
		t.Errorf("GrossMargin: got %.1f, want 92.5", d.GrossMargin)
	}
	if d.NetMargin != 50.0 {
		t.Errorf("NetMargin: got %.1f, want 50.0", d.NetMargin)
	}
	if d.Source != "eastmoney" {
		t.Errorf("source: got %s, want eastmoney", d.Source)
	}
}

func TestParseFinancial_EmptyData(t *testing.T) {
	client := &EastMoneyClient{}
	_, err := client.parseFinancial("sh000001", []byte(`{"data":{}}`))
	if err != nil {
		t.Fatalf("empty data should not error: %v", err)
	}
}
