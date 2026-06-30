package flow

import (
	"fmt"
	"strings"
	"time"

	"robot_yysls/internal/session"
	"robot_yysls/internal/style"
)

type Store interface {
	Get(groupID, userID string) (*session.Session, bool)
	Save(session *session.Session)
	Delete(groupID, userID string)
}

type Coordinator struct {
	Store  Store
	Styles *style.Registry
}

func (c *Coordinator) Start(groupID, userID string, now time.Time) (string, error) {
	if c.Store == nil || c.Styles == nil {
		return "", fmt.Errorf("coordinator requires store and styles")
	}

	sess := session.New(groupID, userID, now)
	c.Store.Save(sess)

	return "请选择流派：\n" + c.styleList(), nil
}

type StyleSelection struct {
	Reply  string
	Styles []style.Config
}

func (c *Coordinator) SelectStyles(groupID, userID, text string, now time.Time) (StyleSelection, error) {
	sess, ok := c.Store.Get(groupID, userID)
	if !ok {
		return StyleSelection{Reply: "请先发送「@机器人 计算毕业率」开始计算。"}, nil
	}
	if sess.State != session.StateChoosingStyles {
		return StyleSelection{Reply: "当前会话不能选择流派，请发送「重新开始」后再试。"}, nil
	}

	names := splitStyleNames(text)
	styles, err := c.Styles.MustResolveNames(names)
	if err != nil {
		return StyleSelection{Reply: "流派名称有误：" + err.Error()}, nil
	}
	sess.State = session.StateCompleted
	sess.UpdatedAt = now
	c.Store.Save(sess)

	var builder strings.Builder
	builder.WriteString("已为你准备以下流派模板：")
	for _, cfg := range styles {
		builder.WriteString("\n")
		builder.WriteString(cfg.Name)
	}
	builder.WriteString("\n请在收到的 xlsx 模板中自行填写属性，毕业率会由表格公式自动计算。")
	return StyleSelection{Reply: builder.String(), Styles: styles}, nil
}

func (c *Coordinator) styleList() string {
	styles := c.Styles.List()
	lines := make([]string, 0, len(styles))
	for _, cfg := range styles {
		lines = append(lines, cfg.Name)
	}
	return strings.Join(lines, "、")
}

func splitStyleNames(text string) []string {
	replacer := strings.NewReplacer("，", " ", ",", " ", "、", " ", "\n", " ", "\t", " ")
	normalized := replacer.Replace(text)
	fields := strings.Fields(normalized)
	return fields
}
