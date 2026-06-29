package style

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

const DefaultTemplateDir = "/home/xiaruyi.ly/workspace-personal/cal_module"

var styleNamePattern = regexp.MustCompile(`^(.+?)110阶`)

func LoadTemplateConfigs(templateDir string) ([]Config, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}

	configs := make([]Config, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xlsx") {
			continue
		}

		styleName, err := StyleNameFromTemplateFile(entry.Name())
		if err != nil {
			return nil, err
		}
		templatePath := filepath.Join(templateDir, entry.Name())
		fields, err := ExtractTemplateFields(templatePath)
		if err != nil {
			return nil, err
		}
		configs = append(configs, Config{
			ID:           styleName,
			Name:         styleName,
			TemplatePath: templatePath,
			Result:       CellRef{Sheet: "期望", Cell: "I16"},
			Fields:       fields,
		})
	}

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Name < configs[j].Name
	})
	return configs, nil
}

func MustLoadDefaultRegistry() (*Registry, error) {
	configs, err := LoadTemplateConfigs(DefaultTemplateDir)
	if err != nil {
		return nil, err
	}
	return NewRegistry(configs)
}

func ExtractTemplateFields(templatePath string) (map[string]FieldConfig, error) {
	file, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fields := make(map[string]FieldConfig)
	addPair := func(labelCell, valueCell string) error {
		label, err := file.GetCellValue("期望", labelCell)
		if err != nil {
			return err
		}
		label = strings.TrimSpace(label)
		if label == "" {
			return nil
		}
		formula, err := file.GetCellFormula("期望", valueCell)
		if err != nil {
			return err
		}
		if formula != "" {
			return nil
		}
		value, err := file.GetCellValue("期望", valueCell)
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			return nil
		}
		addField(fields, label, CellRef{Sheet: "期望", Cell: valueCell}, value)
		return nil
	}

	for row := 8; row <= 24; row++ {
		if err := addPair(fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row)); err != nil {
			return nil, err
		}
	}
	for row := 2; row <= 9; row++ {
		if err := addPair(fmt.Sprintf("H%d", row), fmt.Sprintf("I%d", row)); err != nil {
			return nil, err
		}
	}
	for row := 15; row <= 21; row++ {
		if err := addPair(fmt.Sprintf("C%d", row), fmt.Sprintf("D%d", row)); err != nil {
			return nil, err
		}
		if err := addPair(fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row)); err != nil {
			return nil, err
		}
	}
	for row := 2; row <= 7; row++ {
		if err := addPair(fmt.Sprintf("F%d", row), fmt.Sprintf("G%d", row)); err != nil {
			return nil, err
		}
	}

	for row := 2; row <= 7; row++ {
		rowLabel, err := file.GetCellValue("期望", fmt.Sprintf("A%d", row))
		if err != nil {
			return nil, err
		}
		rowLabel = strings.TrimSpace(rowLabel)
		if rowLabel == "" {
			continue
		}
		for col := 2; col <= 5; col++ {
			headerCell, err := excelize.CoordinatesToCellName(col, 1)
			if err != nil {
				return nil, err
			}
			valueCell, err := excelize.CoordinatesToCellName(col, row)
			if err != nil {
				return nil, err
			}
			header, err := file.GetCellValue("期望", headerCell)
			if err != nil {
				return nil, err
			}
			header = strings.TrimSpace(header)
			if header == "" {
				continue
			}
			formula, err := file.GetCellFormula("期望", valueCell)
			if err != nil {
				return nil, err
			}
			if formula != "" {
				continue
			}
			value, err := file.GetCellValue("期望", valueCell)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			addField(fields, rowLabel+"."+header, CellRef{Sheet: "期望", Cell: valueCell}, value)
		}
	}

	return fields, nil
}

func StyleNameFromTemplateFile(fileName string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	matches := styleNamePattern.FindStringSubmatch(base)
	if len(matches) != 2 {
		return "", fmt.Errorf("cannot extract style name from template file %q", fileName)
	}
	styleName := strings.TrimSpace(matches[1])
	if styleName == "" {
		return "", fmt.Errorf("empty style name in template file %q", fileName)
	}
	return styleName, nil
}

func addField(fields map[string]FieldConfig, name string, cell CellRef, sampleValue string) {
	fieldName := uniqueFieldName(fields, name, cell.Cell)
	fields[fieldName] = FieldConfig{
		Name: fieldName,
		Cell: cell,
		Type: inferFieldType(sampleValue),
	}
}

func uniqueFieldName(fields map[string]FieldConfig, name, cell string) string {
	if _, exists := fields[name]; !exists {
		return name
	}
	return name + "@" + cell
}

func inferFieldType(value string) FieldType {
	trimmed := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if trimmed == "" {
		return FieldTypeText
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return FieldTypeNumber
	}
	return FieldTypeText
}
