package flow

import (
	"strings"
	"testing"
	"time"

	"robot_yysls/internal/session"
	"robot_yysls/internal/style"
)

func TestCoordinatorStartAndSelectStyles(t *testing.T) {
	coordinator := testCoordinator(t)
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)

	startReply, err := coordinator.Start("group", "user", now)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !strings.Contains(startReply, "@user") || !strings.Contains(startReply, "流派A") {
		t.Fatalf("Start() reply = %q", startReply)
	}

	selection, err := coordinator.SelectStyles("group", "user", "流派A 流派B", now)
	if err != nil {
		t.Fatalf("SelectStyles() error = %v", err)
	}
	for _, want := range []string{"@user", "流派A", "流派B", "xlsx 模板", "自行填写属性"} {
		if !strings.Contains(selection.Reply, want) {
			t.Fatalf("SelectStyles() reply missing %q: %q", want, selection.Reply)
		}
	}
	if len(selection.Styles) != 2 {
		t.Fatalf("selected styles = %+v, want 2", selection.Styles)
	}
}

func TestCoordinatorSelectStylesRequiresStart(t *testing.T) {
	coordinator := testCoordinator(t)

	selection, err := coordinator.SelectStyles("group", "user", "流派A", time.Now())
	if err != nil {
		t.Fatalf("SelectStyles() error = %v", err)
	}
	if !strings.Contains(selection.Reply, "请先发送") {
		t.Fatalf("SelectStyles() reply = %q", selection.Reply)
	}
	if len(selection.Styles) != 0 {
		t.Fatalf("selected styles = %+v, want none", selection.Styles)
	}
}

func TestCoordinatorSelectStylesRejectsUnknownStyle(t *testing.T) {
	coordinator := testCoordinator(t)
	now := time.Now()
	if _, err := coordinator.Start("group", "user", now); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	selection, err := coordinator.SelectStyles("group", "user", "不存在", now)
	if err != nil {
		t.Fatalf("SelectStyles() error = %v", err)
	}
	if !strings.Contains(selection.Reply, "流派名称有误") {
		t.Fatalf("SelectStyles() reply = %q", selection.Reply)
	}
	if len(selection.Styles) != 0 {
		t.Fatalf("selected styles = %+v, want none", selection.Styles)
	}
}

func testCoordinator(t *testing.T) *Coordinator {
	t.Helper()

	registry, err := style.NewRegistry([]style.Config{
		{ID: "style_a", Name: "流派A", TemplatePath: "templates/a.xlsx", Result: style.CellRef{Sheet: "结果", Cell: "B2"}},
		{ID: "style_b", Name: "流派B", TemplatePath: "templates/b.xlsx", Result: style.CellRef{Sheet: "结果", Cell: "B2"}},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return &Coordinator{
		Store:  session.NewMemoryStore(),
		Styles: registry,
	}
}
