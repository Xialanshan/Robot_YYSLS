package attrparse

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"robot_yysls/internal/ocr"
)

const (
	FieldOuterAttackMin       = "外功攻击最小值"
	FieldOuterAttackMax       = "外功攻击最大值"
	FieldOuterPenetration     = "外功穿透"
	FieldAttrAttackMin        = "属性攻击最小值"
	FieldAttrAttackMax        = "属性攻击最大值"
	FieldAttrPenetration      = "属性穿透"
	FieldPrecisionRate        = "精准率"
	FieldCritRate             = "会心率"
	FieldInsightRate          = "会意率"
	FieldDirectCritRate       = "直接会心率"
	FieldDirectInsightRate    = "直接会意率"
	FieldCritDamageBonus      = "会心伤害加成"
	FieldInsightDamageBonus   = "会意伤害加成"
	FieldAllWeaponBonus       = "全部武学增效"
	FieldSpecifiedWeaponBonus = "指定武学增效"
	FieldSpecifiedSkillBonus  = "指定技能增效"
	FieldBossDamageBonus      = "对首领单位增伤"
	FieldSingleQishuBonus     = "单体奇术增伤"
	FieldGroupQishuBonus      = "群体奇术增伤"
)

var (
	rangePattern       = regexp.MustCompile(`^([0-9.]+)\s*[-~]\s*([0-9.]+)$`)
	parenthesizedValue = regexp.MustCompile(`\(([0-9.]+)%?\)`)
	numberPattern      = regexp.MustCompile(`[0-9.]+`)
)

type ParsedAttributes map[string]float64

type rowItem struct {
	text string
	x    int
	y    int
}

func Parse(items []ocr.TextItem) ParsedAttributes {
	rows := groupRows(items)
	attrs := ParsedAttributes{}
	for _, row := range rows {
		label, value := splitRow(row)
		addParsedValue(attrs, label, value)
	}
	return attrs
}

func groupRows(items []ocr.TextItem) [][]rowItem {
	rows := make([][]rowItem, 0, len(items))
	for _, item := range items {
		text := normalizeText(item.Text)
		if text == "" {
			continue
		}
		x, y := itemCenter(item)
		placed := false
		for i := range rows {
			if abs(rows[i][0].y-y) <= 12 {
				rows[i] = append(rows[i], rowItem{text: text, x: x, y: y})
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, []rowItem{{text: text, x: x, y: y}})
		}
	}
	for _, row := range rows {
		sort.Slice(row, func(i, j int) bool {
			return row[i].x < row[j].x
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0].y < rows[j][0].y
	})
	return rows
}

func splitRow(row []rowItem) (string, string) {
	if len(row) == 0 {
		return "", ""
	}
	labelParts := make([]string, 0, len(row))
	valueParts := make([]string, 0, len(row))
	for _, item := range row {
		if containsDigit(item.text) {
			valueParts = append(valueParts, item.text)
			continue
		}
		if len(valueParts) == 0 {
			labelParts = append(labelParts, item.text)
		}
	}
	return strings.Join(labelParts, ""), strings.Join(valueParts, "")
}

func addParsedValue(attrs ParsedAttributes, label, rawValue string) {
	field := normalizeLabel(label)
	if field == "" || rawValue == "" {
		return
	}
	value := normalizeValue(rawValue)
	if value == "" {
		return
	}
	if matches := rangePattern.FindStringSubmatch(value); len(matches) == 3 {
		minValue, minErr := strconv.ParseFloat(matches[1], 64)
		maxValue, maxErr := strconv.ParseFloat(matches[2], 64)
		if minErr == nil && maxErr == nil {
			switch field {
			case "外功攻击":
				attrs[FieldOuterAttackMin] = minValue
				attrs[FieldOuterAttackMax] = maxValue
			case "属性攻击":
				attrs[FieldAttrAttackMin] = minValue
				attrs[FieldAttrAttackMax] = maxValue
			}
		}
		return
	}
	if match := numberPattern.FindString(value); match != "" {
		parsed, err := strconv.ParseFloat(match, 64)
		if err != nil {
			return
		}
		switch field {
		case FieldOuterPenetration:
			attrs[FieldOuterPenetration] = parsed
		case FieldAttrPenetration:
			attrs[FieldAttrPenetration] = parsed
		case FieldPrecisionRate:
			attrs[FieldPrecisionRate] = parsed
		case FieldCritRate:
			attrs[FieldCritRate] = parsed
		case FieldInsightRate:
			attrs[FieldInsightRate] = parsed
		case FieldDirectCritRate:
			attrs[FieldDirectCritRate] = parsed
		case FieldDirectInsightRate:
			attrs[FieldDirectInsightRate] = parsed
		case FieldCritDamageBonus:
			attrs[FieldCritDamageBonus] = parsed
		case FieldInsightDamageBonus:
			attrs[FieldInsightDamageBonus] = parsed
		case FieldAllWeaponBonus:
			attrs[FieldAllWeaponBonus] = parsed
		case FieldSpecifiedWeaponBonus:
			attrs[FieldSpecifiedWeaponBonus] = parsed
		case FieldSpecifiedSkillBonus:
			attrs[FieldSpecifiedSkillBonus] = parsed
		case FieldBossDamageBonus:
			attrs[FieldBossDamageBonus] = parsed
		case FieldSingleQishuBonus:
			attrs[FieldSingleQishuBonus] = parsed
		case FieldGroupQishuBonus:
			attrs[FieldGroupQishuBonus] = parsed
		}
	}
}

func normalizeLabel(label string) string {
	label = normalizeText(label)
	switch label {
	case "外功攻击":
		return "外功攻击"
	case "属性攻击":
		return "属性攻击"
	case "外功穿透":
		return FieldOuterPenetration
	case "属攻穿透", "属性穿透":
		return FieldAttrPenetration
	case "精准率":
		return FieldPrecisionRate
	case "会心率":
		return FieldCritRate
	case "会意率":
		return FieldInsightRate
	case "直接会心率":
		return FieldDirectCritRate
	case "直接会意率":
		return FieldDirectInsightRate
	case "会心伤害加成":
		return FieldCritDamageBonus
	case "会意伤害加成":
		return FieldInsightDamageBonus
	case "全部武学增效":
		return FieldAllWeaponBonus
	case "指定武学增效":
		return FieldSpecifiedWeaponBonus
	case "指定技能增效":
		return FieldSpecifiedSkillBonus
	case "对首领单位增伤":
		return FieldBossDamageBonus
	case "单体奇术增伤":
		return FieldSingleQishuBonus
	case "群体奇术增伤":
		return FieldGroupQishuBonus
	default:
		return ""
	}
}

func normalizeValue(value string) string {
	value = normalizeText(value)
	if matches := parenthesizedValue.FindStringSubmatch(value); len(matches) == 2 {
		return matches[1]
	}
	value = strings.TrimSuffix(value, "%")
	return value
}

func normalizeText(text string) string {
	replacer := strings.NewReplacer(
		" ", "",
		"\t", "",
		"：", "",
		":", "",
		"（", "(",
		"）", ")",
		"％", "%",
		"—", "-",
		"–", "-",
	)
	return strings.TrimSpace(replacer.Replace(text))
}

func itemCenter(item ocr.TextItem) (int, int) {
	if len(item.Polygon) == 0 {
		return 0, 0
	}
	var x, y int
	for _, point := range item.Polygon {
		x += point.X
		y += point.Y
	}
	return x / len(item.Polygon), y / len(item.Polygon)
}

func containsDigit(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return r >= '0' && r <= '9'
	}) >= 0
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
