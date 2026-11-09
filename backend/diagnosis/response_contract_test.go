package diagnosis

import (
	"strings"
	"testing"
)

func TestResponseContractDefinesApplicationDiagnosisModel(t *testing.T) {
	instruction := buildResponseInstruction("基础诊断指令")
	for _, term := range []string{"scope_context", "当前诊断范围", "应用模型与设计机制", "应用是业务服务或业务组件，不等于资源", "application.instances", "application.resource_dependencies", "dependencies/dependents", "selector", "binding", "代码仓库", "源码级诊断", "数据库", "缓存", "消息队列", "注册中心", "配置中心", "ReAct/Loop", "每轮重新规划", "跨层证据链", "数据库内部 ID", "最窄工具", "实例级工具", "container_name", "运行时守护进程", "内置 Skill：应用诊断", "建立配置基线", "实例优先", "形成并验证假设", "汇总"} {
		if !strings.Contains(instruction, term) {
			t.Fatalf("default diagnosis contract missing %q", term)
		}
	}
}

func TestResponseContractOmitsApplicationSkillWithoutApplicationContext(t *testing.T) {
	instruction := buildResponseInstructionFor("普通资源诊断", false)
	if strings.Contains(instruction, "内置 Skill：应用诊断") {
		t.Fatal("application skill should not be sent for a non-application diagnosis")
	}
}
