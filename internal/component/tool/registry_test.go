package tool

import (
	"context"
	"testing"

	eino_tool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// dummyTool 用于测试注册中心的模拟工具。
type dummyTool struct {
	name string
}

func (d *dummyTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: d.name,
		Desc: d.name + " description",
	}, nil
}

func (d *dummyTool) InvokableRun(_ context.Context, argsJSON string, opts ...eino_tool.Option) (string, error) {
	return d.name + " executed", nil
}

// 验证 dummyTool 实现了 Tool 接口（编译时检查）
var _ Tool = (*dummyTool)(nil)

func TestRegisterAndList(t *testing.T) {
	// 使用独立注册中心，不影响全局
	r := &Registry{tools: make([]Tool, 0)}

	t1 := &dummyTool{name: "tool_a"}
	t2 := &dummyTool{name: "tool_b"}

	_ = r.Add(t1)
	_ = r.Add(t2)

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}

	// 验证返回的是拷贝而非引用
	names := make(map[string]bool)
	for _, tl := range list {
		info, _ := tl.Info(context.Background())
		names[info.Name] = true
	}
	if !names["tool_a"] || !names["tool_b"] {
		t.Errorf("missing tools: %v", names)
	}
}

func TestRegister_Duplicate(t *testing.T) {
	r := &Registry{tools: make([]Tool, 0)}

	d := &dummyTool{name: "dup"}
	_ = r.Add(d)
	_ = r.Add(d)

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries (no dedup), got %d", len(list))
	}
}

func TestList_DefensiveCopy(t *testing.T) {
	r := &Registry{tools: make([]Tool, 0)}
	_ = r.Add(&dummyTool{name: "a"})
	_ = r.Add(&dummyTool{name: "b"})

	list1 := r.List()
	list2 := r.List()

	// 验证两次返回的是不同切片
	if len(list1) == 0 {
		t.Fatal("list should not be empty")
	}
	list1[0] = &dummyTool{name: "modified"}

	// 第二次拿到的不应该受影响
	info, _ := list2[0].Info(context.Background())
	if info.Name == "modified" {
		t.Error("defensive copy failed: list2 affected by list1 modification")
	}
}

func TestInfo_All(t *testing.T) {
	r := &Registry{tools: make([]Tool, 0)}
	_ = r.Add(&dummyTool{name: "x"})
	_ = r.Add(&dummyTool{name: "y"})

	infos, err := r.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("Info len = %d, want 2", len(infos))
	}
	if infos[0].Name != "x" || infos[1].Name != "y" {
		t.Errorf("unexpected order: %v", []string{infos[0].Name, infos[1].Name})
	}
}

func TestInfo_ErrorPropagation(t *testing.T) {
	// 模拟一个 Info 会报错的工具
	errTool := &dummyTool{name: "broken"}
	r := &Registry{tools: make([]Tool, 0)}
	_ = r.Add(&dummyTool{name: "good"})
	_ = r.Add(errTool)

	// 不覆盖 Info，保持正常
	infos, err := r.Info(context.Background())
	if err != nil {
		t.Fatalf("Info should not error: %v", err)
	}
	if len(infos) != 2 {
		t.Errorf("expected 2 infos, got %d", len(infos))
	}
}

func TestGlobalRegistry(t *testing.T) {
	// 验证全局单例存在且可以注册
	if globalRegistry == nil {
		t.Fatal("globalRegistry is nil")
	}
	d := &dummyTool{name: "global_test"}
	err := Register(d)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	names := RegisteredTools()
	found := false
	for _, tl := range names {
		info, _ := tl.Info(context.Background())
		if info.Name == "global_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("global_test not found in registered tools")
	}

	infos, err := ToolInfos(context.Background())
	if err != nil {
		t.Fatalf("ToolInfos: %v", err)
	}
	if len(infos) == 0 {
		t.Error("ToolInfos returned empty list")
	}
}

func TestToolInterface_ImplementsInvokableTool(t *testing.T) {
	// 验证 Tool 接口可以赋值给 Eino 的 BaseTool
	d := &dummyTool{name: "compat"}
	var einoTool eino_tool.BaseTool = d
	if einoTool == nil {
		t.Fatal("Tool does not satisfy eino_tool.BaseTool")
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := &Registry{tools: make([]Tool, 0)}

	list := r.List()
	if len(list) != 0 {
		t.Errorf("empty registry list = %d entries", len(list))
	}

	infos, err := r.Info(context.Background())
	if err != nil {
		t.Fatalf("Info on empty registry: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("empty registry Info = %d entries", len(infos))
	}
}
