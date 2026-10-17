package core

import (
	"regexp"
	"strings"
	"unicode"
)

// Element 为解析中需要保留的标签元素。
type Element struct {
	Tag       string
	Attrs     map[string]string
	Text      string // 可见文本（已展平）
	ParentIdx int    // 父元素在 doc.Elements 中的下标，-1 表示无
}

// Doc 为一次轻量解析的结果。
type Doc struct {
	Elements []Element
	Text     string // 全部可见文本
	Visible  string // 可见 HTML 序列化（保留原始开标签，供精确正则匹配）
	Empty    bool   // 无任何可见子元素
}

var (
	attrRe    = regexp.MustCompile(`(?:\s+([a-zA-Z_:][a-zA-Z0-9_:.\-]*)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+)))?)`)
	tagNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*`)
)

// voidElements 不会包含子内容，直接视为自闭合。
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// removedElements 的内容整体不进入可见文本与可见 HTML。
var removedElements = map[string]bool{
	"script": true, "style": true, "template": true, "noscript": true,
	"textarea": true, "head": true,
}

// trackedTags 解析时收集元素属性的标签集合（覆盖证据判定与签到参数提取所需）。
var trackedTags = map[string]bool{
	"a": true, "form": true, "input": true, "button": true, "meta": true,
	"div": true, "td": true, "span": true, "b": true, "img": true,
	"table": true, "shark-icon-button": true,
}

// hiddenAttr 判定元素是否被显式隐藏。
func hiddenAttr(attrs map[string]string) bool {
	if _, ok := attrs["hidden"]; ok {
		return true
	}
	if strings.EqualFold(attrs["aria-hidden"], "true") {
		return true
	}
	style := attrs["style"]
	if style != "" && (regexp.MustCompile(`(?i)display\s*:\s*none`).MatchString(style) ||
		regexp.MustCompile(`(?i)visibility\s*:\s*hidden`).MatchString(style)) {
		return true
	}
	return false
}

func parseAttrs(segment string) map[string]string {
	attrs := map[string]string{}
	for _, m := range attrRe.FindAllStringSubmatch(segment, -1) {
		name := m[1]
		if name == "" {
			continue
		}
		value := m[2]
		if value == "" {
			value = m[3]
		}
		if value == "" {
			value = m[4]
		}
		attrs[strings.ToLower(name)] = value
	}
	return attrs
}

type openNode struct {
	tag     string
	rawOpen string // 原始开标签文本（含尖括号）
	removed bool
	text    string // 累积可见文本
	el      *Element
	idx     int // 在 doc.Elements 中的下标，-1 表示未追踪
}

// ParseDoc 对 HTML 做轻量解析（保留隐藏元素，供签到参数提取使用）：
//   - 剔除 script/style/template/noscript/textarea/head 内容；
//   - 保留可见元素并收集属性与可见文本；
//   - 生成保留原始开标签的可见 HTML 序列化。
func ParseDoc(source string) *Doc { return parse(source, false) }

// ParseVisible 与 ParseDoc 相同，但额外剔除显式隐藏元素与 hidden 输入框，
// 供证据判定使用（与原版 visible_html 语义一致）。
func ParseVisible(source string) *Doc { return parse(source, true) }

func parse(source string, removeHidden bool) *Doc {
	doc := &Doc{}
	stack := []*openNode{}
	pos := 0
	n := len(source)
	var textBuf strings.Builder
	var visible strings.Builder

	flushText := func() {
		if textBuf.Len() == 0 {
			return
		}
		raw := textBuf.String()
		textBuf.Reset()
		// 任一祖先被移除则该文本整体丢弃
		removed := false
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].removed {
				removed = true
				break
			}
		}
		if removed {
			return
		}
		decoded := decodeEntities(raw)
		if strings.TrimSpace(raw) != "" {
			doc.Text += decoded
			// 同步到所有未移除祖先，使元素 Text 等价于 itertext 的累积文本
			for _, node := range stack {
				if !node.removed {
					node.text += decoded
					if node.el != nil {
						node.el.Text += decoded
					}
					if node.idx >= 0 && node.idx < len(doc.Elements) {
						doc.Elements[node.idx].Text += decoded
					}
				}
			}
		}
		visible.WriteString(raw)
	}

	for pos < n {
		idx := strings.IndexByte(source[pos:], '<')
		if idx < 0 {
			textBuf.WriteString(source[pos:])
			break
		}
		idx += pos
		if idx > pos {
			textBuf.WriteString(source[pos:idx])
		}
		if strings.HasPrefix(source[idx:], "<!--") {
			// 注释直接跳过
			end := strings.Index(source[idx+4:], "-->")
			if end < 0 {
				break
			}
			pos = idx + 4 + end + 3
			continue
		}
		if strings.HasPrefix(source[idx:], "<![CDATA[") {
			end := strings.Index(source[idx+9:], "]]>")
			if end < 0 {
				break
			}
			pos = idx + 9 + end + 3
			continue
		}
		gt := strings.IndexByte(source[idx:], '>')
		if gt < 0 {
			textBuf.WriteString(source[pos:])
			break
		}
		segment := source[idx : idx+gt+1]
		pos = idx + gt + 1
		inner := segment[1 : len(segment)-1]
		if strings.HasPrefix(inner, "!") {
			// doctype 等声明
			continue
		}
		if strings.HasPrefix(inner, "/") {
			// 闭合标签
			name := strings.ToLower(strings.TrimSpace(tagNameRe.FindString(strings.TrimSpace(inner[1:]))))
			rawClose := segment
			flushText()
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				if top.tag == name {
					if !top.removed {
						visible.WriteString(rawClose)
					}
					stack = stack[:len(stack)-1]
				}
				// 不匹配的闭合标签忽略
			}
			continue
		}
		// 开标签
		name := strings.ToLower(tagNameRe.FindString(strings.TrimSpace(inner)))
		if name == "" {
			textBuf.WriteString(segment)
			continue
		}
		attrs := parseAttrs(inner)
		selfClose := strings.HasSuffix(strings.TrimSpace(inner), "/") || voidElements[name]
		removed := removedElements[name] || (removeHidden && hiddenAttr(attrs)) ||
			(removeHidden && name == "input" && strings.EqualFold(attrs["type"], "hidden"))
		var tracked *Element
		elIdx := -1
		if !removed && trackedTags[name] {
			parent := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if !stack[i].removed && stack[i].idx >= 0 {
					parent = stack[i].idx
					break
				}
			}
			tracked = &Element{Tag: name, Attrs: attrs, ParentIdx: parent}
			elIdx = len(doc.Elements)
			doc.Elements = append(doc.Elements, *tracked)
		}
		if !removed {
			doc.Empty = false
		}
		flushText()
		if !removed {
			visible.WriteString(segment)
		}
		if selfClose || voidElements[name] {
			continue
		}
		stack = append(stack, &openNode{tag: name, rawOpen: segment, removed: removed, el: tracked, idx: elIdx})
	}
	flushText()
	doc.Visible = visible.String()
	return doc
}

// decodeEntities 解码常见与数字实体，itertext 语义足够。
func decodeEntities(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = numEntityRe.ReplaceAllStringFunc(s, func(m string) string {
		digits := m[2 : len(m)-1]
		code := 0
		for _, r := range digits {
			code = code*10 + int(r-'0')
		}
		return string(rune(code))
	})
	return s
}

var numEntityRe = regexp.MustCompile(`&#\d+;`)

// CompactText 去除全部空白，用于标签/文案比对。
func CompactText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// StripTags 移除全部标签，返回纯文本。
func StripTags(s string) string {
	var b strings.Builder
	pos := 0
	n := len(s)
	for pos < n {
		idx := strings.IndexByte(s[pos:], '<')
		if idx < 0 {
			b.WriteString(s[pos:])
			break
		}
		idx += pos
		if idx > pos {
			b.WriteString(s[pos:idx])
		}
		gt := strings.IndexByte(s[idx:], '>')
		if gt < 0 {
			b.WriteString(s[idx:])
			break
		}
		pos = idx + gt + 1
	}
	return decodeEntities(b.String())
}

// ElementAttr 在文档中查找第一个 tag 下属性 name 值等于 want 的元素，返回其全部属性。
func (d *Doc) ElementAttr(tag, name, want string) (map[string]string, bool) {
	for _, e := range d.Elements {
		if e.Tag != tag {
			continue
		}
		if v, ok := e.Attrs[name]; ok && v == want {
			return e.Attrs, true
		}
	}
	return nil, false
}

// ElementsAttrContains 收集指定标签中某属性值包含关键字的元素。
func (d *Doc) ElementsAttrContains(tag, name, keyword string) []Element {
	out := []Element{}
	for _, e := range d.Elements {
		if e.Tag != tag {
			continue
		}
		if strings.Contains(strings.ToLower(e.Attrs[name]), strings.ToLower(keyword)) {
			out = append(out, e)
		}
	}
	return out
}

// DescendantsWithAncestorClass 返回标签为 tag、且任一祖先元素 class 包含关键字的元素。
func (d *Doc) DescendantsWithAncestorClass(tag, classKeyword string) []Element {
	ancestor := func(idx int) bool {
		for idx >= 0 {
			p := d.Elements[idx]
			if strings.Contains(strings.ToLower(p.Attrs["class"]), strings.ToLower(classKeyword)) {
				return true
			}
			idx = p.ParentIdx
		}
		return false
	}
	out := []Element{}
	for i := range d.Elements {
		if d.Elements[i].Tag == tag && ancestor(d.Elements[i].ParentIdx) {
			out = append(out, d.Elements[i])
		}
	}
	return out
}
