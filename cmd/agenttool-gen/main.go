// agenttool-gen —— @agenttool 注解代码生成器
//
// 用法:
//
//	go run cmd/agenttool-gen/main.go \
//	  -dir internal/service \
//	  -out internal/component/tool/gen_service_tools.go \
//	  -pkg tool \
//	  -service-import github.com/MrLuoooooo/MrLuoooooagent/internal/service
//
// 注解格式:
//
//	// @agenttool name="tool_name" desc="工具描述"
//	// @param paramName 类型 参数描述 [required]
//
// 生成文件包含:
//   - 每个方法一个 Tool 结构体（实现 Info + InvokableRun）
//   - Provider 函数（接收 service 依赖，调用 Register）
//   - 编译期接口检查
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// toolMethod 描述一个被 @agenttool 标记的方法。
type toolMethod struct {
	GoName   string // 方法名（Go标识符，如 GetFeedbackStats）
	ToolName string // 工具名（如 get_feedback_stats）
	Desc     string // 工具描述
	Params   []toolParam
	// 接收者
	RecvType   string // 接收者类型名（如 FeedbackService）
	RecvPkg    string // 接收者所在包名（如 service）
	ImportPath string // 接收者包的导入路径
	// 返回值
	ReturnType     string // 第一个返回类型（空串表示只有 error）
	ReturnIsString bool   // 第一个返回是 string
}

// toolParam 描述一个工具参数。
type toolParam struct {
	Name     string // 参数名
	GoType   string // Go 类型（如 string, int, float64, []string）
	Desc     string // 参数描述
	Required bool   // 是否必填
}

// ===== 配置 =====

var (
	sourceDir      string
	outputFile     string
	outputPkg      string
	serviceImport  string
)

// ===== 入口 =====

func main() {
	flag.StringVar(&sourceDir, "dir", "", "源代码目录（必填）")
	flag.StringVar(&outputFile, "out", "", "输出文件路径（必填）")
	flag.StringVar(&outputPkg, "pkg", "tool", "输出包名（默认 tool）")
	flag.StringVar(&serviceImport, "service-import", "", "Service 包的导入路径（必填）")
	flag.Parse()

	if sourceDir == "" || outputFile == "" || serviceImport == "" {
		fmt.Fprintln(os.Stderr, "用法: agenttool-gen -dir <src> -out <file> -service-import <path>")
		os.Exit(1)
	}

	methods, err := extractTools(sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析失败: %v\n", err)
		os.Exit(1)
	}
	if len(methods) == 0 {
		fmt.Println("未发现 @agenttool 注解，跳过生成。")
		return
	}

	// 按接收者+方法名排序
	sort.Slice(methods, func(i, j int) bool {
		if methods[i].RecvType != methods[j].RecvType {
			return methods[i].RecvType < methods[j].RecvType
		}
		return methods[i].GoName < methods[j].GoName
	})

	fmt.Printf("发现 %d 个 @agenttool 方法:\n", len(methods))
	for _, m := range methods {
		fmt.Printf("  %s.%s → tool=%s\n", m.RecvType, m.GoName, m.ToolName)
	}

	if err := generate(outputFile, outputPkg, serviceImport, methods); err != nil {
		fmt.Fprintf(os.Stderr, "生成失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已生成: %s\n", outputFile)
}

// ===== AST 解析 =====

func extractTools(dir string) ([]toolMethod, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse dir %s: %w", dir, err)
	}

	var methods []toolMethod
	for pkgName, pkg := range pkgs {
		for _, file := range pkg.Files {
			ms := extractFromFile(pkgName, file, fset)
			methods = append(methods, ms...)
		}
	}
	return methods, nil
}

func extractFromFile(pkgName string, file *ast.File, fset *token.FileSet) []toolMethod {
	var methods []toolMethod

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		if fn.Doc == nil {
			continue
		}

		m, err := parseAnnotation(fn, pkgName)
		if err != nil || m == nil {
			continue // 无注解或解析失败，跳过
		}
		methods = append(methods, *m)
	}
	return methods
}

// ===== 注解解析 =====

var (
	toolRe   = regexp.MustCompile(`@agenttool\s+name="([^"]+)"\s+desc="([^"]+)"`)
	paramRe  = regexp.MustCompile(`@param\s+(\w+)\s+(\S+)\s+(.+?)(?:\s+required)?\s*$`)
)

func parseAnnotation(fn *ast.FuncDecl, pkgName string) (*toolMethod, error) {
	var commentText string
	for _, c := range fn.Doc.List {
		commentText += c.Text + "\n"
	}

	match := toolRe.FindStringSubmatch(commentText)
	if match == nil {
		return nil, nil // 不是 @agenttool 注解
	}

	m := &toolMethod{
		ToolName: match[1],
		Desc:     match[2],
		GoName:   fn.Name.Name,
	}

	// 解析接收者
	if len(fn.Recv.List) > 0 {
		m.RecvType, m.RecvPkg = parseRecv(fn.Recv.List[0], pkgName)
	}

	// 解析 @param 行
	for _, line := range strings.Split(commentText, "\n") {
		pm := paramRe.FindStringSubmatch(strings.TrimSpace(line))
		if pm == nil {
			continue
		}
		required := strings.HasSuffix(strings.TrimSpace(line), "required")
		m.Params = append(m.Params, toolParam{
			Name:     pm[1],
			GoType:   pm[2],
			Desc:     strings.TrimSpace(pm[3]),
			Required: required,
		})
	}

	// 解析返回值
	// 三种情况:
	//   (T, error) → 使用 T 作为 ReturnType
	//   error      → ReturnType 留空（无业务返回值）
	//   无返回值   → ReturnType 留空
	if fn.Type.Results != nil && len(fn.Type.Results.List) == 2 {
		firstRet := fn.Type.Results.List[0]
		m.ReturnType = typeToString(firstRet.Type)
		m.ReturnIsString = m.ReturnType == "string"
	}
	// 1 个返回值 → 只能是 error，跳过（ReturnType 保持空串）

	return m, nil
}

func parseRecv(field *ast.Field, pkgName string) (typeName, recvPkg string) {
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name, pkgName
		}
		if sel, ok := t.X.(*ast.SelectorExpr); ok {
			return sel.Sel.Name, sel.X.(*ast.Ident).Name
		}
	case *ast.Ident:
		return t.Name, pkgName
	}
	return "Unknown", pkgName
}

func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// ===== 代码生成 =====

var genTemplate = template.Must(template.New("tools").Funcs(template.FuncMap{
	"schemaType":    schemaType,
	"goToJSONField": goToJSONField,
	"toTitle":       toTitle,
	"isSlice":       isSlice,
	"sliceElem":     sliceElem,
	"toSnake":       toSnake,
}).Parse(`// Code generated by agenttool-gen. DO NOT EDIT.
// source: {{.SourceDir}}

package {{.Package}}

import (
	"context"
	"encoding/json"
	"fmt"

	eino_tool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"{{.ServiceImport}}"
)

{{range .Methods}}
// {{.GoName}}Tool —— @agenttool 自动生成
type {{.GoName}}Tool struct {
	svc *{{.RecvPkg}}.{{.RecvType}}
}

func (t *{{.GoName}}Tool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "{{.ToolName}}",
		Desc: {{printf "%q" .Desc}},
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
{{- range .Params}}
			{{printf "%q" .Name}}: {
				Type:     {{schemaType .GoType}},
				Desc:     {{printf "%q" .Desc}},
				Required: {{.Required}},
				{{- if isSlice .GoType}}
				ElemInfo: &schema.ParameterInfo{Type: {{schemaType (sliceElem .GoType)}}},
				{{- end}}
			},
{{- end}}
		}),
	}, nil
}

func (t *{{.GoName}}Tool) InvokableRun(ctx context.Context, argsJSON string, opts ...eino_tool.Option) (string, error) {
	var args struct {
{{- range .Params}}
		{{.Name | toTitle}} {{.GoType}} ` + "`" + `json:"{{goToJSONField .Name}}"` + "`" + `
{{- end}}
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("{{.ToolName}}: parse args: %w", err)
	}
{{- if eq .ReturnType ""}}
	if err := t.svc.{{.GoName}}(ctx{{range .Params}}, args.{{.Name | toTitle}}{{end}}); err != nil {
		return "", err
	}
	return "{}", nil
{{- else if .ReturnIsString}}
	result, err := t.svc.{{.GoName}}(ctx{{range .Params}}, args.{{.Name | toTitle}}{{end}})
	if err != nil {
		return "", err
	}
	return result, nil
{{- else}}
	result, err := t.svc.{{.GoName}}(ctx{{range .Params}}, args.{{.Name | toTitle}}{{end}})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("{{.ToolName}}: marshal result: %w", err)
	}
	return string(b), nil
{{- end}}
}

// Provide{{.GoName}}Tool 供 Fx DI 使用，内部自动调用 Register
func Provide{{.GoName}}Tool(svc *{{.RecvPkg}}.{{.RecvType}}) Tool {
	t := &{{.GoName}}Tool{svc: svc}
	Register(t)
	return t
}

var _ eino_tool.InvokableTool = (*{{.GoName}}Tool)(nil)
{{end}}
`))

func generate(outFile, pkg, svcImport string, methods []toolMethod) error {
	for i := range methods {
		methods[i].RecvPkg = pkgNameFromImport(svcImport)
		methods[i].ImportPath = svcImport
	}

	f, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer f.Close()

	data := struct {
		SourceDir     string
		Timestamp     string
		Package       string
		ServiceImport string
		Methods       []toolMethod
	}{
		SourceDir:     sourceDir,
		Timestamp:     "auto-generated",
		Package:       pkg,
		ServiceImport: svcImport,
		Methods:       methods,
	}

	return genTemplate.Execute(f, data)
}

// ===== 辅助函数 =====

func pkgNameFromImport(importPath string) string {
	return filepath.Base(importPath)
}

func schemaType(goType string) string {
	switch goType {
	case "string":
		return "schema.String"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "schema.Integer"
	case "float32", "float64":
		return "schema.Number"
	case "bool":
		return "schema.Boolean"
	default:
		if strings.HasPrefix(goType, "[]") {
			return "schema.Array"
		}
		return "schema.Object"
	}
}

func goToJSONField(name string) string {
	// 保持原样（Go 参数名即 JSON 字段名）
	return name
}

// toTitle 首字母大写（替代废弃的 strings.Title）
func toTitle(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 32
	}
	return string(r)
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func isSlice(goType string) bool {
	return strings.HasPrefix(goType, "[]")
}

func sliceElem(goType string) string {
	return strings.TrimPrefix(goType, "[]")
}
