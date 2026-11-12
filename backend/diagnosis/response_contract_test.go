package diagnosis

import (
	"strings"
	"testing"
)

func TestResponseContractDefinesApplicationDiagnosisModel(t *testing.T) {
	instruction := buildResponseInstructionFor("基础诊断指令", true, true)
	for _, term := range []string{"application_context", "selected_resources", "代码仓库", "合理路径均已检查", "应用是业务服务/组件", "application.instances", "application.resource_dependencies", "dependencies/dependents", "selector", "binding", "next_offset", "done=true", "complete=true", "truncated=true", "read_observation", "text_truncated", "未读范围", "数据库", "缓存", "消息队列", "注册/配置中心", "ReAct/Loop", "数据库内部 ID", "container_name", "真实 Docker ID", "配置快照", "active 实例", "运行时连接状态", "源码", "可信度", "未查范围"} {
		if !strings.Contains(instruction, term) {
			t.Fatalf("default diagnosis contract missing %q", term)
		}
	}
	if strings.Contains(instruction, "scope_context") {
		t.Fatal("diagnosis prompt references the legacy scope_context field, which is not sent to the model")
	}
}

func TestResponseContractResolvesDiagnosticTargetsFromContext(t *testing.T) {
	instruction := buildResponseInstructionFor("基础诊断指令", true, true)
	for _, term := range []string{"对象定位顺序：先读本轮实际提供的 application_context、selected_resources", "不要为“确认”它是否存在而调用范围查询", "不要先把名称当成应用查询", "diagnosis_application_context 只用于用户明确询问应用", "合理路径均已检查仍无匹配"} {
		if !strings.Contains(instruction, term) {
			t.Fatalf("diagnosis target resolution contract missing %q", term)
		}
	}
}

func TestResponseContractKeepsPromptFocused(t *testing.T) {
	instruction := buildResponseInstructionFor("基础诊断指令", true, true)
	if strings.Contains(instruction, "推荐图标词典") || strings.Contains(instruction, "📋") {
		t.Fatal("response contract should not prescribe a detailed icon vocabulary")
	}
	if len(instruction) > 6500 {
		t.Fatalf("built-in diagnosis instruction grew unexpectedly: %d bytes", len(instruction))
	}
}

func TestResponseContractOmitsApplicationSkillWithoutApplicationContext(t *testing.T) {
	instruction := buildResponseInstructionFor("普通资源诊断", false, false)
	if strings.Contains(instruction, "内置 Skill：应用诊断") {
		t.Fatal("application skill should not be sent for a non-application diagnosis")
	}
}

func TestResponseContractLoadsSkillsOnlyForRelevantContext(t *testing.T) {
	base := buildResponseInstructionFor("资源诊断", false, false)
	if strings.Contains(base, "内置 Skill：应用诊断") || strings.Contains(base, "内置 Skill：代码仓库诊断") {
		t.Fatal("generic resource diagnosis received an unrelated skill")
	}
	app := buildResponseInstructionFor("应用诊断", true, false)
	if !strings.Contains(app, "内置 Skill：应用诊断") || strings.Contains(app, "repository_search") {
		t.Fatal("application-only diagnosis should include app rules without repository read protocol")
	}
	repository := buildResponseInstructionFor("仓库诊断", false, true)
	if !strings.Contains(repository, "内置 Skill：代码仓库诊断") || strings.Contains(repository, "内置 Skill：应用诊断") {
		t.Fatal("repository-only diagnosis should include repository rules without app workflow")
	}
}
