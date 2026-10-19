package sseparser

import (
	"strings"
	"testing"
)

func TestApplyPatchModelDILFallbackMarkdown(t *testing.T) {
	state := &PatchState{}
	ApplyPatch(state, "/message/id", "replace", "msg-old")
	if !ApplyPatch(state, "/message/metadata/model_dil_v2/fallbackMarkdown", "append", "普通") {
		t.Fatal("first fallbackMarkdown patch was not applied")
	}
	ApplyPatch(state, "/message/metadata/model_dil_v2/fallbackMarkdown", "append", "文本")

	if got := ModelDILFallback(state.Response); got != "普通文本" {
		t.Fatalf("fallbackMarkdown = %q, want 普通文本", got)
	}

	ApplyPatch(state, "/message/metadata/model_dil_v2", "replace", map[string]interface{}{
		"fallbackMarkdown": "完整替换",
	})
	if got := ModelDILFallback(state.Response); got != "完整替换" {
		t.Fatalf("object fallbackMarkdown = %q, want 完整替换", got)
	}

	ApplyPatch(state, "/message/id", "replace", "msg-new")
	if got := ModelDILFallback(state.Response); got != "" {
		t.Fatalf("new message retained old fallbackMarkdown: %q", got)
	}
}

func TestDegradeDILToMarkdownGeneric(t *testing.T) {
	input := `## 推荐

<box gap={3}>
  <row align="start">
    <AsyncImage ref="turn1product0" aspectRatio="3:4"/>
    <box flex="1">
      **<Entity ref="turn1product0" category="product" value={turn1product0.title}/>**
      <title size="xl">{"<0.59 秒"}</title>
      静态说明。<Cite refs={["turn1search0"]}/>
    </box>
  </row>
</box>
{@body const [purpose,setPurpose] = DIL.useState("wall")}
<button onClick={()=>GenUI.issueNewTurn("继续")}>按用途推荐</button>

比较符保持：1 < 2，自动链接保持：<https://example.com/a>。
内联代码保持：` + "`<box>code</box>`" + `。

` + "```jsx" + `
<box>{turn1.value}</box>
` + "```" + `
`

	got := DegradeDILToMarkdown(input)
	outsideCode := strings.ReplaceAll(got, "`<box>code</box>`", "")
	outsideCode = strings.ReplaceAll(outsideCode, "```jsx\n<box>{turn1.value}</box>\n```", "")
	for _, leaked := range []string{"<box", "</box", "<row", "<AsyncImage", "<Entity", "<title", "<Cite", "<button", "{@body", "GenUI.issueNewTurn", "turn1product0.title"} {
		if strings.Contains(outsideCode, leaked) {
			t.Errorf("degraded output leaked %q:\n%s", leaked, got)
		}
	}
	for _, preserved := range []string{
		"## 推荐",
		"<0.59 秒",
		"静态说明。",
		"按用途推荐",
		"1 < 2",
		"<https://example.com/a>",
		"`<box>code</box>`",
		"```jsx\n<box>{turn1.value}</box>\n```",
	} {
		if !strings.Contains(got, preserved) {
			t.Errorf("degraded output lost %q:\n%s", preserved, got)
		}
	}
}

func TestDegradeDILLeavesOrdinaryMarkdownUnchanged(t *testing.T) {
	input := "普通 **Markdown**，集合 {x, y}，小数 {3.14}，比较 3 < 5。\n\n`<Widget/>`\n\n~~~html\n<Widget>{value}</Widget>\n~~~\n"
	if got := DegradeDILToMarkdown(input); got != input {
		t.Fatalf("ordinary markdown changed:\n got: %q\nwant: %q", got, input)
	}
}

func TestDILStreamPipelineSplitSyntax(t *testing.T) {
	pipeline := &DILStreamPipeline{}
	var got strings.Builder

	got.WriteString(pipeline.Feed("前文<Asyn"))
	if got.String() != "前文" {
		t.Fatalf("partial tag leaked: %q", got.String())
	}
	got.WriteString(pipeline.Feed("cImage query={\"x > y\"}/>"))
	got.WriteString(pipeline.Feed("\n<box>正文"))
	got.WriteString(pipeline.Feed("</bo"))
	got.WriteString(pipeline.Feed("x>尾{@body"))
	got.WriteString(pipeline.Flush())

	if got.String() != "前文\n正文尾" {
		t.Fatalf("stream degradation = %q, want %q", got.String(), "前文\n正文尾")
	}
}

func TestDILStreamPipelineHoldsSingleDelimiter(t *testing.T) {
	pipeline := &DILStreamPipeline{}
	if got := pipeline.Feed("前文<"); got != "前文" {
		t.Fatalf("single angle delimiter leaked: %q", got)
	}
	if got := pipeline.Feed("UnknownWidget/>后文{"); got != "后文" {
		t.Fatalf("single brace delimiter handling = %q, want 后文", got)
	}
	if got := pipeline.Feed(`"literal"}`); got != "literal" {
		t.Fatalf("static expression handling = %q, want literal", got)
	}
}
