package db

import (
	"encoding/json"
	"testing"
)

// TestFlexFloat_UnmarshalJSON 覆盖东财混用类型：number / "-" / null / 非法字符串。
func TestFlexFloat_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"plain number", `{"v": 123.45}`, 123.45},
		{"integer number", `{"v": 1200}`, 1200},
		{"dash string", `{"v": "-"}`, 0},
		{"empty string", `{"v": ""}`, 0},
		{"json null", `{"v": null}`, 0},
		{"garbage string", `{"v": "abc"}`, 0},
		{"quoted number", `{"v": "88.8"}`, 88.8},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out struct {
				V FlexFloat `json:"v"`
			}
			if err := json.Unmarshal([]byte(c.in), &out); err != nil {
				t.Fatalf("unmarshal %s: %v", c.in, err)
			}
			if float64(out.V) != c.want {
				t.Fatalf("got %v, want %v", float64(out.V), c.want)
			}
		})
	}
}

// TestParseStockPage_MixedTypes 端到端：同一批数据里 number 与 "-" 混存，解析不报错且数值正确。
func TestParseStockPage_MixedTypes(t *testing.T) {
	data := []byte(`{"data":{"total":2,"diff":[
		{"f12":"600519","f14":"贵州茅台","f100":"酿酒行业","f20":1800000000000,"f115":22.5,"f117":8.1},
		{"f12":"000778","f14":"新兴铸管","f100":"钢铁行业","f20":"-","f115":"-","f117":"-"}
	]}}`)

	list, total, err := parseStockPage(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("total=%d len=%d", total, len(list))
	}
	if list[0].MarketCap != 18000 { // 1.8e12 / 1e8 = 18000 亿
		t.Fatalf("market cap: got %v, want 18000", list[0].MarketCap)
	}
	if list[1].MarketCap != 0 || list[1].PE != 0 || list[1].PB != 0 {
		t.Fatalf("dirty values should be 0: %+v", list[1])
	}
}

// TestParseStockPage_NullData 越界页（data:null）返回空列表而非报错，分页循环靠它正常终止。
func TestParseStockPage_NullData(t *testing.T) {
	list, total, err := parseStockPage([]byte(`{"data":null}`))
	if err != nil {
		t.Fatalf("parse null page: %v", err)
	}
	if len(list) != 0 || total != 0 {
		t.Fatalf("want empty page, got len=%d total=%d", len(list), total)
	}
}
