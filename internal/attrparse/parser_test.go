package attrparse

import (
	"testing"

	"robot_yysls/internal/ocr"
)

func TestParseScreenshotAttributes(t *testing.T) {
	items := []ocr.TextItem{
		textBox("外功攻击", 100, 100),
		textBox("1543-5071", 300, 100),
		textBox("属性攻击", 100, 130),
		textBox("557-1274", 300, 130),
		textBox("精准率", 100, 160),
		textBox("108.4%(82.7%)", 300, 160),
		textBox("会心率", 100, 190),
		textBox("55.3%(22.6%)", 300, 190),
		textBox("会意率", 100, 220),
		textBox("92.9%(37.9%)", 300, 220),
	}

	got := Parse(items)

	assertFloat(t, got, FieldOuterAttackMin, 1543)
	assertFloat(t, got, FieldOuterAttackMax, 5071)
	assertFloat(t, got, FieldAttrAttackMin, 557)
	assertFloat(t, got, FieldAttrAttackMax, 1274)
	assertFloat(t, got, FieldPrecisionRate, 82.7)
	assertFloat(t, got, FieldCritRate, 22.6)
	assertFloat(t, got, FieldInsightRate, 37.9)
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

func assertFloat(t *testing.T, values ParsedAttributes, key string, want float64) {
	t.Helper()
	if values[key] != want {
		t.Fatalf("%s = %v, want %v; all=%+v", key, values[key], want, values)
	}
}
