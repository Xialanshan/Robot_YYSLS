package qqapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"robot_yysls/internal/attrparse"
	"robot_yysls/internal/ocr"
	"robot_yysls/internal/session"
	"robot_yysls/internal/style"
)

const defaultGeneratedTemplateTTL = time.Hour

type OutboundTemplate struct {
	Name string
	Path string
}

type OCRHandleResult struct {
	Handled   bool
	Reply     string
	Templates []OutboundTemplate
}

type OCRCommandHandler interface {
	Handle(ctx context.Context, groupID, userID, text string, images []ImageReference, now time.Time) (OCRHandleResult, error)
}

type OCRSessionStore interface {
	Get(groupID, userID string) (*session.Session, bool)
	Save(session *session.Session)
	Delete(groupID, userID string)
}

type OCRProcessor struct {
	Store      OCRSessionStore
	Styles     *style.Registry
	OCR        ocr.Provider
	HTTPClient *http.Client
	WorkDir    string
	TTL        time.Duration
	AfterFunc  func(time.Duration, func()) *time.Timer
}

func (p *OCRProcessor) Handle(ctx context.Context, groupID, userID, text string, images []ImageReference, now time.Time) (OCRHandleResult, error) {
	if p == nil {
		return OCRHandleResult{}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := p.cleanupExpired(now); err != nil {
		return OCRHandleResult{}, err
	}
	switch {
	case strings.Contains(text, "OCR计算"):
		return p.handleCalculate(ctx, groupID, userID, text, images, now)
	case strings.Contains(text, "发我模板"):
		return p.handleSendTemplates(groupID, userID, text, now)
	default:
		return OCRHandleResult{}, nil
	}
}

func (p *OCRProcessor) handleCalculate(ctx context.Context, groupID, userID, text string, images []ImageReference, now time.Time) (OCRHandleResult, error) {
	if p.Styles == nil || p.Store == nil {
		return OCRHandleResult{Handled: true, Reply: "OCR 计算服务尚未完成初始化，请稍后再试。"}, nil
	}
	if p.OCR == nil {
		return OCRHandleResult{Handled: true, Reply: "OCR 计算尚未启用，请先在服务器配置 OCR_PROVIDER=paddle，并安装本地 PaddleOCR 依赖。"}, nil
	}
	styleNames := parseCommandStyles(text, "OCR计算")
	if len(styleNames) == 0 {
		return OCRHandleResult{Handled: true, Reply: "请在「OCR计算」后附带流派名称，例如：@机器人 OCR计算 鸣金虹"}, nil
	}
	if len(images) == 0 {
		return OCRHandleResult{Handled: true, Reply: "请在同一条 @ 消息里附带截图，例如：@机器人 OCR计算 鸣金虹 + 2 张属性截图。"}, nil
	}
	styles, err := p.Styles.MustResolveNames(styleNames)
	if err != nil {
		return OCRHandleResult{Handled: true, Reply: "流派名称有误：" + err.Error()}, nil
	}

	ocrItems := make([]ocr.TextItem, 0, 64)
	for _, image := range images {
		data, err := p.downloadImage(ctx, image)
		if err != nil {
			return OCRHandleResult{Handled: true, Reply: "截图下载失败，请重新发送截图。错误：" + err.Error()}, nil
		}
		result, err := p.OCR.Recognize(ctx, ocr.ImageInput{Data: data})
		if err != nil {
			return OCRHandleResult{Handled: true, Reply: "OCR 识别失败，请稍后重试。错误：" + err.Error()}, nil
		}
		ocrItems = append(ocrItems, result.Items...)
	}
	attrs := attrparse.Parse(ocrItems)
	if len(attrs) == 0 {
		return OCRHandleResult{Handled: true, Reply: "未能从截图中识别到可用属性，请确认截图清晰且包含完整属性列表后重试。"}, nil
	}
	if missing := missingRequiredAttributes(attrs); len(missing) > 0 {
		return OCRHandleResult{
			Handled: true,
			Reply:   "截图识别不完整，缺少这些关键属性：" + strings.Join(missing, "、") + "。请重新发送包含完整数据的截图。",
		}, nil
	}

	workDir := p.workDir()
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return OCRHandleResult{}, err
	}
	if sess, ok := p.Store.Get(groupID, userID); ok {
		_ = removeGeneratedFiles(sess.GeneratedTemplates)
	}
	runDir := filepath.Join(workDir, sanitizeFileName(groupID+"_"+userID+"_"+now.Format("20060102_150405")))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return OCRHandleResult{}, err
	}
	if err := os.Chtimes(runDir, now, now); err != nil {
		return OCRHandleResult{}, err
	}

	generated := make(map[string]string, len(styles))
	lines := make([]string, 0, len(styles)+4)
	lines = append(lines, "已完成 OCR 计算：")
	for _, cfg := range styles {
		path, value, err := p.generateWorkbook(runDir, cfg, attrs, now)
		if err != nil {
			lines = append(lines, cfg.Name+"：计算失败（"+err.Error()+"）")
			continue
		}
		generated[cfg.Name] = path
		lines = append(lines, cfg.Name+"："+formatGraduationRate(value))
	}
	if len(generated) == 0 {
		return OCRHandleResult{Handled: true, Reply: strings.Join(lines, "\n")}, nil
	}

	sess, ok := p.Store.Get(groupID, userID)
	if !ok {
		sess = session.New(groupID, userID, now)
	}
	sess.GeneratedTemplates = generated
	sess.GeneratedExpiresAt = now.Add(p.ttl())
	sess.UpdatedAt = now
	p.Store.Save(sess)
	runDirCopy := runDir
	p.afterFunc()(p.ttl(), func() {
		_ = os.RemoveAll(runDirCopy)
	})

	storedStyles := make([]string, 0, len(generated))
	for name := range generated {
		storedStyles = append(storedStyles, name)
	}
	sort.Strings(storedStyles)
	lines = append(lines, "")
	lines = append(lines, "如需查看填写后的模板，请在 1 小时内发送：")
	lines = append(lines, "发我模板 "+strings.Join(storedStyles, " "))
	return OCRHandleResult{Handled: true, Reply: strings.Join(lines, "\n")}, nil
}

func (p *OCRProcessor) handleSendTemplates(groupID, userID, text string, now time.Time) (OCRHandleResult, error) {
	if p.Store == nil {
		return OCRHandleResult{Handled: true, Reply: "当前没有可发送的 OCR 模板副本。"}, nil
	}
	sess, ok := p.Store.Get(groupID, userID)
	if !ok || len(sess.GeneratedTemplates) == 0 {
		return OCRHandleResult{Handled: true, Reply: "当前没有可发送的 OCR 模板副本，请先发送「@机器人 OCR计算 流派名」并附带截图。"}, nil
	}
	if !sess.GeneratedExpiresAt.IsZero() && !now.Before(sess.GeneratedExpiresAt) {
		_ = removeGeneratedFiles(sess.GeneratedTemplates)
		sess.GeneratedTemplates = nil
		sess.GeneratedExpiresAt = time.Time{}
		sess.UpdatedAt = now
		p.Store.Save(sess)
		return OCRHandleResult{Handled: true, Reply: "填写后的模板副本已超过 1 小时有效期，已自动删除。请重新执行 OCR 计算。"}, nil
	}

	requested := parseCommandStyles(text, "发我模板")
	templates := make([]OutboundTemplate, 0, len(sess.GeneratedTemplates))
	if len(requested) == 0 {
		names := make([]string, 0, len(sess.GeneratedTemplates))
		for name := range sess.GeneratedTemplates {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			templates = append(templates, OutboundTemplate{Name: name, Path: sess.GeneratedTemplates[name]})
		}
	} else {
		for _, name := range requested {
			path, exists := sess.GeneratedTemplates[name]
			if !exists {
				continue
			}
			templates = append(templates, OutboundTemplate{Name: name, Path: path})
		}
	}
	if len(templates) == 0 {
		return OCRHandleResult{Handled: true, Reply: "未找到对应流派的 OCR 模板副本，请确认流派名称，或重新执行 OCR 计算。"}, nil
	}
	var builder strings.Builder
	builder.WriteString("已为你发送填写后的模板副本：")
	for _, template := range templates {
		builder.WriteString("\n")
		builder.WriteString(template.Name)
	}
	builder.WriteString("\n这些副本会在生成 1 小时后自动删除。")
	return OCRHandleResult{Handled: true, Reply: builder.String(), Templates: templates}, nil
}

func (p *OCRProcessor) downloadImage(ctx context.Context, image ImageReference) ([]byte, error) {
	if image.URL == "" {
		return nil, fmt.Errorf("image URL is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func (p *OCRProcessor) generateWorkbook(runDir string, cfg style.Config, attrs attrparse.ParsedAttributes, now time.Time) (string, string, error) {
	dstPath := filepath.Join(runDir, filepath.Base(cfg.TemplatePath))
	src, err := os.ReadFile(cfg.TemplatePath)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(dstPath, src, 0o644); err != nil {
		return "", "", err
	}

	file, err := excelize.OpenFile(dstPath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	for fieldName, value := range buildTemplateValues(cfg, attrs) {
		field, ok := cfg.Fields[fieldName]
		if !ok {
			continue
		}
		if err := file.SetCellValue(field.Cell.Sheet, field.Cell.Cell, value); err != nil {
			return "", "", err
		}
	}
	if err := file.Save(); err != nil {
		return "", "", err
	}
	if err := os.Chtimes(dstPath, now, now); err != nil {
		return "", "", err
	}
	result, err := file.CalcCellValue(cfg.Result.Sheet, cfg.Result.Cell)
	if err != nil {
		result, err = file.GetCellValue(cfg.Result.Sheet, cfg.Result.Cell)
		if err != nil {
			return "", "", err
		}
	}
	return dstPath, strings.TrimSpace(result), nil
}

func buildTemplateValues(cfg style.Config, attrs attrparse.ParsedAttributes) map[string]float64 {
	values := make(map[string]float64)
	assign := func(fieldName, attrName string) {
		if _, ok := cfg.Fields[fieldName]; !ok {
			return
		}
		if value, ok := attrs[attrName]; ok {
			values[fieldName] = value
		}
	}

	assign("外功.最小攻击", attrparse.FieldOuterAttackMin)
	assign("外功.最大攻击", attrparse.FieldOuterAttackMax)
	assign("外功.穿透", attrparse.FieldOuterPenetration)
	assign("精准率", attrparse.FieldPrecisionRate)
	assign("会心率", attrparse.FieldCritRate)
	assign("会意率", attrparse.FieldInsightRate)
	assign("直接会心率", attrparse.FieldDirectCritRate)
	assign("直接会意率", attrparse.FieldDirectInsightRate)
	assign("会心伤害加成", attrparse.FieldCritDamageBonus)
	assign("会意伤害加成", attrparse.FieldInsightDamageBonus)
	assign("全武增", attrparse.FieldAllWeaponBonus)
	assign("首领增", attrparse.FieldBossDamageBonus)
	assign("单体奇术增", attrparse.FieldSingleQishuBonus)
	assign("群体奇术增", attrparse.FieldGroupQishuBonus)

	stylePrefix := styleAttackPrefix(cfg.Name)
	if stylePrefix != "" {
		assign(stylePrefix+".最小攻击", attrparse.FieldAttrAttackMin)
		assign(stylePrefix+".最大攻击", attrparse.FieldAttrAttackMax)
		assign(stylePrefix+".穿透", attrparse.FieldAttrPenetration)
	}

	if weaponBonus, ok := attrs[attrparse.FieldSpecifiedWeaponBonus]; ok {
		for name := range cfg.Fields {
			if isWeaponBonusField(name) {
				values[name] = weaponBonus
			}
		}
	}
	return values
}

func isWeaponBonusField(name string) bool {
	if !strings.HasSuffix(name, "增") {
		return false
	}
	switch name {
	case "全武增", "首领增", "单体奇术增", "群体奇术增":
		return false
	default:
		return true
	}
}

func styleAttackPrefix(styleName string) string {
	switch {
	case strings.Contains(styleName, "鸣金"):
		return "鸣金"
	case strings.Contains(styleName, "裂石"):
		return "裂石"
	case strings.Contains(styleName, "牵丝"):
		return "牵丝"
	case strings.Contains(styleName, "破竹"):
		return "破竹"
	default:
		return ""
	}
}

func missingRequiredAttributes(attrs attrparse.ParsedAttributes) []string {
	required := []struct {
		label string
		key   string
	}{
		{label: "外功攻击", key: attrparse.FieldOuterAttackMin},
		{label: "属性攻击", key: attrparse.FieldAttrAttackMin},
		{label: "精准率", key: attrparse.FieldPrecisionRate},
		{label: "会心率", key: attrparse.FieldCritRate},
		{label: "会意率", key: attrparse.FieldInsightRate},
	}
	missing := make([]string, 0, len(required))
	for _, item := range required {
		if _, ok := attrs[item.key]; !ok {
			missing = append(missing, item.label)
		}
	}
	return missing
}

func parseCommandStyles(text, keyword string) []string {
	text = strings.TrimSpace(text)
	text = strings.Replace(text, keyword, "", 1)
	replacer := strings.NewReplacer("，", " ", ",", " ", "、", " ", "\n", " ", "\t", " ")
	fields := strings.Fields(replacer.Replace(text))
	return fields
}

func formatGraduationRate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "计算完成，但未能读到毕业率单元格结果"
	}
	return trimmed
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", "\n", "_", "\r", "_", "\t", "_", " ", "_")
	return replacer.Replace(name)
}

func removeGeneratedFiles(files map[string]string) error {
	var firstErr error
	removedDirs := make(map[string]struct{})
	for _, path := range files {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
		dir := filepath.Dir(path)
		if _, seen := removedDirs[dir]; seen {
			continue
		}
		removedDirs[dir] = struct{}{}
		_ = os.Remove(dir)
	}
	return firstErr
}

func (p *OCRProcessor) cleanupExpired(now time.Time) error {
	workDir := p.workDir()
	entries, err := os.ReadDir(workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Add(p.ttl()).After(now) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(workDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (p *OCRProcessor) httpClient() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (p *OCRProcessor) workDir() string {
	if p.WorkDir != "" {
		return p.WorkDir
	}
	return filepath.Join(os.TempDir(), "robot_yysls", "ocr")
}

func (p *OCRProcessor) ttl() time.Duration {
	if p.TTL > 0 {
		return p.TTL
	}
	return defaultGeneratedTemplateTTL
}

func (p *OCRProcessor) afterFunc() func(time.Duration, func()) *time.Timer {
	if p.AfterFunc != nil {
		return p.AfterFunc
	}
	return time.AfterFunc
}
