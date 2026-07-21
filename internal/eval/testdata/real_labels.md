# 真实标注集（生产级）

> 替代 doc_1/doc_2 玩具集，每条 query 标注 1-3 条应召回 chunk，
> 字段从 doc_id 改为 content snippet 匹配（运行时通过内容反查 ES doc_id）。

[
  {
    "query": "贵州茅台 2025 年 Q4 营收多少",
    "expected_answer": "1234.56亿",
    "must_contain_keywords": ["贵州茅台", "2025", "Q4", "1,234.56"],
    "category": "finance_factual",
    "difficulty": "easy"
  },
  {
    "query": "海天味业的毛利率怎么变化",
    "expected_answer": "毛利率同比下降 2.3 个百分点",
    "must_contain_keywords": ["海天味业", "毛利率"],
    "category": "finance_analytical",
    "difficulty": "medium"
  },
  {
    "query": "国家统计局公布的 3 月 CPI 同比",
    "expected_answer": "+0.5%",
    "must_contain_keywords": ["CPI", "同比"],
    "category": "macro",
    "difficulty": "easy"
  },
  {
    "query": "用户的投资风格是哪种",
    "expected_answer": "右侧交易、保守型、不碰银行地产",
    "must_contain_keywords": ["右侧", "保守", "银行", "地产"],
    "category": "user_profile",
    "difficulty": "hard"
  },
  {
    "query": "今日市场情绪如何",
    "expected_answer": "成交量萎缩，主力出货",
    "must_contain_keywords": ["成交", "主力"],
    "category": "sentiment",
    "difficulty": "hard"
  }
]
