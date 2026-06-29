package style

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestStyleNameFromTemplateFile(t *testing.T) {
	tests := map[string]string{
		"牵丝玉110阶竞速轴属性毕业率进阶计算器1.0.xlsx":  "牵丝玉",
		"牵丝翊110阶竞速轴属性毕业率进阶计算器1.0.xlsx":  "牵丝翊",
		"破竹尘110阶竞速轴属性毕业率进阶计算器1.0.xlsx":  "破竹尘",
		"破竹风110阶竞速轴属性毕业率进阶计算器1.0.xlsx":  "破竹风",
		"裂石威110阶竞速轴属性毕业率进阶计算器1.0.xlsx":  "裂石威",
		"裂石钧110阶竞速轴属性毕业率进阶计算器1.13.xlsx": "裂石钧",
		"鸣金影110阶竞速轴属性毕业率进阶计算器1.1.xlsx":  "鸣金影",
		"鸣金虹110阶竞速轴属性毕业率进阶计算器1.1.xlsx":  "鸣金虹",
	}

	for fileName, want := range tests {
		got, err := StyleNameFromTemplateFile(fileName)
		if err != nil {
			t.Fatalf("StyleNameFromTemplateFile(%q) error = %v", fileName, err)
		}
		if got != want {
			t.Fatalf("StyleNameFromTemplateFile(%q) = %q, want %q", fileName, got, want)
		}
	}
}

func TestExtractTemplateFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "template.xlsx")
	file := excelize.NewFile()
	defer file.Close()
	if _, err := file.NewSheet("期望"); err != nil {
		t.Fatalf("NewSheet() error = %v", err)
	}
	values := map[string]any{
		"B1":  "最小攻击",
		"C1":  "最大攻击",
		"D1":  "穿透",
		"A2":  "外功",
		"B2":  2133.7,
		"C2":  5409.5,
		"D2":  63.5,
		"A23": "食物加成",
		"B23": 200,
		"A24": "食物加成",
		"B24": 400,
		"C16": "威",
		"D16": "√",
		"H16": "毕业率",
	}
	for cell, value := range values {
		if err := file.SetCellValue("期望", cell, value); err != nil {
			t.Fatalf("SetCellValue(%s) error = %v", cell, err)
		}
	}
	if err := file.SetCellFormula("期望", "I16", "I12/100"); err != nil {
		t.Fatalf("SetCellFormula() error = %v", err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatalf("SaveAs() error = %v", err)
	}

	fields, err := ExtractTemplateFields(path)
	if err != nil {
		t.Fatalf("ExtractTemplateFields() error = %v", err)
	}

	assertFieldCell(t, fields, "外功.最小攻击", "B2")
	assertFieldCell(t, fields, "外功.最大攻击", "C2")
	assertFieldCell(t, fields, "外功.穿透", "D2")
	assertFieldCell(t, fields, "威", "D16")
	assertFieldCell(t, fields, "食物加成", "B23")
	assertFieldCell(t, fields, "食物加成@B24", "B24")
	if _, exists := fields["毕业率"]; exists {
		t.Fatalf("ExtractTemplateFields() should not include formula result field")
	}
}

func assertFieldCell(t *testing.T, fields map[string]FieldConfig, name, wantCell string) {
	t.Helper()

	field, ok := fields[name]
	if !ok {
		t.Fatalf("field %q not found in %+v", name, fields)
	}
	if field.Cell.Cell != wantCell {
		t.Fatalf("field %q cell = %q, want %q", name, field.Cell.Cell, wantCell)
	}
}
