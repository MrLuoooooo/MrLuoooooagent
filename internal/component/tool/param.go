package tool

import (
	"encoding/json"
	"fmt"
)

// ArgsValidator 参数自检接口。
// 实现此接口的类型在 JSON 反序列化后会自动调用 Validate。
// 设计意图：
//   - ISP: 单方法接口，不强制工具暴露内部结构
//   - LSP: ParseArgs 对任何 ArgsValidator 实现行为一致
type ArgsValidator interface {
	Validate() error
}

// ParseArgs 泛型参数解析器。
// 统一处理 json.Unmarshal + ArgsValidator.Validate。
// 设计意图：
//   - SRP: 只负责参数解析与校验，不参与业务逻辑
//   - OCP: 新增工具参数类型无需改动此函数
//   - DIP: 依赖 ArgsValidator 接口而非具体类型
func ParseArgs[T ArgsValidator](argsJSON string) (T, error) {
	var args T
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return args, fmt.Errorf("参数解析失败: %w", err)
	}
	if err := args.Validate(); err != nil {
		return args, err
	}
	return args, nil
}
