package diagnosis

import (
	"strings"
	"testing"
)

func TestResponseContractDefinesApplicationDiagnosisModel(t *testing.T) {
	instruction := buildResponseInstruction("基础诊断指令")
	for _, term := range []string{"scope_context", "application_context", "selected_resources", "代码仓库", "合理路径均已检查", "应用是业务服务/组件", "application.instances", "application.resource_dependencies", "dependencies/dependents", "selector", "binding", "next_offset", "done=true", "complete=true", "truncated=true", "read_observation", "text_truncated", "未读范围", "数据库", "缓存", "消息队列", "注册/配置中心", "ReAct/Loop", "数据库内部 ID", "container_name", "真实 container_id", "内置 Skill：应用诊断", "配置基线", "active 实例", "运行时平台状态", "源码", "可信度", "未检查项"} {
		if !strings.Contains(instruction, term) {
			t.Fatalf("default diagnosis contract missing %q", term)
		}
	}
}

func TestResponseContractResolvesDiagnosticTargetsFromContext(t *testing.T) {
	instruction := buildResponseInstruction("基础诊断指令")
	for _, term := range []string{"先用本轮 application_context、selected_resources", "不要为了确认已选对象是否存在而调用范围查询工具", "代码仓库“git”不能误判为应用", "精确候选缺失或不匹配时", "只有合理路径均已检查仍无匹配时"} {
		if !strings.Contains(instruction, term) {
			t.Fatalf("diagnosis target resolution contract missing %q", term)
		}
	}
}

func TestResponseContractKeepsPromptFocused(t *testing.T) {
	instruction := buildResponseInstruction("基础诊断指令")
	if strings.Contains(instruction, "推荐图标词典") || strings.Contains(instruction, "📋") {
		t.Fatal("response contract should not prescribe a detailed icon vocabulary")
	}
	if len(instruction) > 6500 {
		t.Fatalf("built-in diagnosis instruction grew unexpectedly: %d bytes", len(instruction))
	}
}

func TestResponseContractOmitsApplicationSkillWithoutApplicationContext(t *testing.T) {
	instruction := buildResponseInstructionFor("普通资源诊断", false)
	if strings.Contains(instruction, "内置 Skill：应用诊断") {
		t.Fatal("application skill should not be sent for a non-application diagnosis")
	}
}
