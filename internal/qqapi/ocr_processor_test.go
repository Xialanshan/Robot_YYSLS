package qqapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"robot_yysls/internal/ocr"
	"robot_yysls/internal/session"
	"robot_yysls/internal/style"
)

func TestOCRProcessorCalculateAndFetchTemplates(t *testing.T) {
	now := time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC)
	templatePath := buildOCRTemplate(t)
	registry, err := style.NewRegistry([]style.Config{{
		ID:           "鸣金虹",
		Name:         "鸣金虹",
		TemplatePath: templatePath,
		Result:       style.CellRef{Sheet: "期望", Cell: "I16"},
		Fields: map[string]style.FieldConfig{
			"外功.最小攻击": {Name: "外功.最小攻击", Cell: style.CellRef{Sheet: "期望", Cell: "B2"}},
			"外功.最大攻击": {Name: "外功.最大攻击", Cell: style.CellRef{Sheet: "期望", Cell: "C2"}},
			"外功.穿透":   {Name: "外功.穿透", Cell: style.CellRef{Sheet: "期望", Cell: "D2"}},
			"鸣金.最小攻击": {Name: "鸣金.最小攻击", Cell: style.CellRef{Sheet: "期望", Cell: "B3"}},
			"鸣金.最大攻击": {Name: "鸣金.最大攻击", Cell: style.CellRef{Sheet: "期望", Cell: "C3"}},
			"鸣金.穿透":   {Name: "鸣金.穿透", Cell: style.CellRef{Sheet: "期望", Cell: "D3"}},
			"精准率":     {Name: "精准率", Cell: style.CellRef{Sheet: "期望", Cell: "B8"}},
			"会心率":     {Name: "会心率", Cell: style.CellRef{Sheet: "期望", Cell: "B9"}},
			"会意率":     {Name: "会意率", Cell: style.CellRef{Sheet: "期望", Cell: "B10"}},
			"直接会心率":   {Name: "直接会心率", Cell: style.CellRef{Sheet: "期望", Cell: "B11"}},
			"直接会意率":   {Name: "直接会意率", Cell: style.CellRef{Sheet: "期望", Cell: "B12"}},
			"会心伤害加成":  {Name: "会心伤害加成", Cell: style.CellRef{Sheet: "期望", Cell: "B13"}},
			"会意伤害加成":  {Name: "会意伤害加成", Cell: style.CellRef{Sheet: "期望", Cell: "B14"}},
			"全武增":     {Name: "全武增", Cell: style.CellRef{Sheet: "期望", Cell: "B15"}},
			"首领增":     {Name: "首领增", Cell: style.CellRef{Sheet: "期望", Cell: "B16"}},
			"剑增":      {Name: "剑增", Cell: style.CellRef{Sheet: "期望", Cell: "B17"}},
			"枪增":      {Name: "枪增", Cell: style.CellRef{Sheet: "期望", Cell: "B18"}},
			"单体奇术增":   {Name: "单体奇术增", Cell: style.CellRef{Sheet: "期望", Cell: "B19"}},
			"群体奇术增":   {Name: "群体奇术增", Cell: style.CellRef{Sheet: "期望", Cell: "B20"}},
		},
	}})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-image"))
	}))
	defer imageServer.Close()

	store := session.NewMemoryStore()
	workDir := t.TempDir()
	processor := &OCRProcessor{
		Store:  store,
		Styles: registry,
		OCR: ocr.FakeProvider{Result: ocr.Result{
			Items: []ocr.TextItem{
				textBox("外功攻击", 100, 100),
				textBox("100-200", 300, 100),
				textBox("属性攻击", 100, 130),
				textBox("30-40", 300, 130),
				textBox("外功穿透", 100, 160),
				textBox("5.5", 300, 160),
				textBox("属攻穿透", 100, 190),
				textBox("6.5", 300, 190),
				textBox("精准率", 100, 220),
				textBox("108.4%(82.7%)", 300, 220),
				textBox("会心率", 100, 250),
				textBox("55.3%(22.6%)", 300, 250),
				textBox("会意率", 100, 280),
				textBox("92.9%(37.9%)", 300, 280),
				textBox("直接会心率", 100, 310),
				textBox("4.6%", 300, 310),
				textBox("直接会意率", 100, 340),
				textBox("2.3%", 300, 340),
				textBox("会心伤害加成", 100, 370),
				textBox("50.0%", 300, 370),
				textBox("会意伤害加成", 100, 400),
				textBox("40.2%", 300, 400),
				textBox("全部武学增效", 100, 430),
				textBox("6.9%", 300, 430),
				textBox("指定武学增效", 100, 460),
				textBox("7.0%", 300, 460),
				textBox("对首领单位增伤", 100, 490),
				textBox("7.2%", 300, 490),
				textBox("单体奇术增伤", 100, 520),
				textBox("26.78%", 300, 520),
				textBox("群体奇术增伤", 100, 550),
				textBox("0.0%", 300, 550),
			},
		}},
		HTTPClient: imageServer.Client(),
		WorkDir:    workDir,
		TTL:        time.Hour,
	}

	result, err := processor.Handle(context.Background(), "group", "user", "OCR计算 鸣金虹", []ImageReference{{URL: imageServer.URL + "/1.png"}}, now)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !result.Handled || !strings.Contains(result.Reply, "鸣金虹") || !strings.Contains(result.Reply, "发我模板 鸣金虹") {
		t.Fatalf("result = %+v", result)
	}

	sess, ok := store.Get("group", "user")
	if !ok || len(sess.GeneratedTemplates) != 1 {
		t.Fatalf("session = %+v", sess)
	}
	generatedPath := sess.GeneratedTemplates["鸣金虹"]
	if filepath.Base(generatedPath) != "鸣金虹.xlsx" {
		t.Fatalf("generated template name = %q", filepath.Base(generatedPath))
	}
	if _, err := os.Stat(generatedPath); err != nil {
		t.Fatalf("generated template missing: %v", err)
	}
	if !strings.Contains(result.Reply, "鸣金虹：模板已生成，请以模板内最终毕业率为准") {
		t.Fatalf("result reply = %q", result.Reply)
	}

	file, err := excelize.OpenFile(generatedPath)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer file.Close()
	resultFormula, err := file.GetCellFormula("期望", "I16")
	if err != nil {
		t.Fatalf("GetCellFormula(I16) error = %v", err)
	}
	if resultFormula == "" {
		t.Fatal("expected result formula to remain in generated workbook")
	}
	calcProps, err := file.GetCalcProps()
	if err != nil {
		t.Fatalf("GetCalcProps() error = %v", err)
	}
	if calcProps.CalcMode == nil || *calcProps.CalcMode != "auto" {
		t.Fatalf("calc mode = %+v", calcProps.CalcMode)
	}
	if calcProps.FullCalcOnLoad == nil || !*calcProps.FullCalcOnLoad {
		t.Fatalf("fullCalcOnLoad = %+v", calcProps.FullCalcOnLoad)
	}
	if calcProps.ForceFullCalc == nil || !*calcProps.ForceFullCalc {
		t.Fatalf("forceFullCalc = %+v", calcProps.ForceFullCalc)
	}
	if calcProps.CalcOnSave == nil || !*calcProps.CalcOnSave {
		t.Fatalf("calcOnSave = %+v", calcProps.CalcOnSave)
	}
	if calcProps.CalcCompleted != nil && *calcProps.CalcCompleted {
		t.Fatalf("calcCompleted = %+v", calcProps.CalcCompleted)
	}
	assertCell(t, file, "期望", "B2", "100")
	assertCell(t, file, "期望", "C2", "200")
	assertCell(t, file, "期望", "D2", "5.5")
	assertCell(t, file, "期望", "B3", "30")
	assertCell(t, file, "期望", "C3", "40")
	assertCell(t, file, "期望", "D3", "6.5")
	assertCell(t, file, "期望", "B8", "0.827")
	assertCell(t, file, "期望", "B9", "0.226")
	assertCell(t, file, "期望", "B10", "0.379")
	assertCell(t, file, "期望", "B11", "0.046")
	assertCell(t, file, "期望", "B12", "0.023")
	assertCell(t, file, "期望", "B13", "0.5")
	assertCell(t, file, "期望", "B14", "0.402")
	assertCell(t, file, "期望", "B15", "0.069")
	assertCell(t, file, "期望", "B16", "0.072")
	assertCell(t, file, "期望", "B17", "0.07")
	assertCell(t, file, "期望", "B18", "0.07")
	assertCell(t, file, "期望", "B19", "0.2678")
	assertCell(t, file, "期望", "B20", "0")

	sendResult, err := processor.Handle(context.Background(), "group", "user", "发我模板 鸣金虹", nil, now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("Handle(fetch) error = %v", err)
	}
	if !sendResult.Handled || len(sendResult.Templates) != 1 || sendResult.Templates[0].Path != generatedPath {
		t.Fatalf("sendResult = %+v", sendResult)
	}

	expiredResult, err := processor.Handle(context.Background(), "group", "user", "发我模板 鸣金虹", nil, now.Add(61*time.Minute))
	if err != nil {
		t.Fatalf("Handle(expired) error = %v", err)
	}
	if !strings.Contains(expiredResult.Reply, "已超过 1 小时有效期") {
		t.Fatalf("expiredResult = %+v", expiredResult)
	}
	if _, err := os.Stat(generatedPath); !os.IsNotExist(err) {
		t.Fatalf("generated template should be deleted, stat err = %v", err)
	}
}

func buildOCRTemplate(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "鸣金虹110阶竞速轴属性毕业率进阶计算器1.1.xlsx")
	file := excelize.NewFile()
	file.SetSheetName("Sheet1", "期望")
	for _, cell := range []string{"B2", "C2", "D2", "B3", "C3", "D3", "B8", "B9", "B10", "B11", "B12", "B13", "B14", "B15", "B16", "B17", "B18", "B19", "B20"} {
		if err := file.SetCellValue("期望", cell, 0); err != nil {
			t.Fatalf("SetCellValue(%s) error = %v", cell, err)
		}
	}
	if err := file.SetCellFormula("期望", "I16", "SUM(B2,C2,D2,B3,C3,D3,B8,B9,B10,B11,B12,B13,B14,B15,B16,B17,B18,B19,B20)"); err != nil {
		t.Fatalf("SetCellFormula() error = %v", err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatalf("SaveAs() error = %v", err)
	}
	return path
}

func assertCell(t *testing.T, file *excelize.File, sheet, cell, want string) {
	t.Helper()
	value, err := file.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatalf("GetCellValue(%s) error = %v", cell, err)
	}
	if value != want {
		t.Fatalf("%s = %q, want %q", cell, value, want)
	}
}

func textBox(text string, x, y int) ocr.TextItem {
	return ocr.TextItem{
		Text: text,
		Polygon: []ocr.Point{
			{X: x - 10, Y: y - 5},
			{X: x + 10, Y: y - 5},
			{X: x + 10, Y: y + 5},
			{X: x - 10, Y: y + 5},
		},
	}
}
