"""
MCP Stock Server - 股票数据查询
使用官方 mcp 库 (pip install mcp)
"""
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("stock-server")

@mcp.tool()
def get_stock_price(code: str) -> str:
    """查询股票实时价格（模拟数据，可替换为真实 API）
    
    Args:
        code: 股票代码，如 600519
    """
    prices = {"600519": 1820.50, "000858": 168.30, "300750": 210.80}
    price = prices.get(code.replace("sh", "").replace("sz", ""), 100.00)
    return f"{code} 最新价: ¥{price}"

@mcp.tool()
def list_favorite_stocks() -> str:
    """列出当前项目的关注股票列表"""
    return "600519 贵州茅台 — 白酒龙头，高ROE\n000858 五粮液 — 白酒老二，估值合理\n300750 宁德时代 — 电池龙头，新质生产力"

if __name__ == "__main__":
    mcp.run()
