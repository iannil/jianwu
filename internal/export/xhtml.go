package export

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// md is the shared markdown engine. Raw HTML stays escaped (safe mode is the
// default) so output remains well-formed XML — the XHTML requirement for
// untrusted LLM-generated bodies. The footnote renderer is overridden with
// EPUB3 semantics. Immutable after init.
var md = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.Footnote),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
		renderer.WithNodeRenderers(util.Prioritized(&epubFootnoteRenderer{}, 10)),
	),
)

// RenderXHTML converts chapter markdown to an XHTML fragment (body content
// only, no html/head wrapper).
func RenderXHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), nil
}

// epubFootnoteRenderer replaces goldmark's default footnote HTML with EPUB3
// semantics: inline refs become noteref anchors, definitions become per-note
// asides in one end-of-chapter footnotes section, with ARIA roles for
// accessibility. goldmark applies node renderers from the highest priority
// number down and later registrations overwrite earlier ones, so a lower
// number (10 < the extension's 500) wins.
type epubFootnoteRenderer struct{}

// RegisterFuncs registers the footnote node renderer overrides.
func (r *epubFootnoteRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(extast.KindFootnoteLink, r.renderFootnoteLink)
	reg.Register(extast.KindFootnoteBacklink, r.renderFootnoteBacklink)
	reg.Register(extast.KindFootnoteList, r.renderFootnoteList)
	reg.Register(extast.KindFootnote, r.renderFootnote)
}

// fnrefID returns the unique id for one inline reference. The first
// reference to note N is fnref-N; later ones append an occurrence suffix
// (the AST carries RefIndex for exactly this). Footnote indices are 1-based.
func fnrefID(index, refIndex int) string {
	id := fmt.Sprintf("fnref-%d", index)
	if refIndex > 0 {
		id = fmt.Sprintf("fnref-%dx%d", index, refIndex)
	}
	return id
}

func (r *epubFootnoteRenderer) renderFootnoteLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.FootnoteLink)
	fmt.Fprintf(w, `<sup class="fn-ref" id="%s"><a epub:type="noteref" role="doc-noteref" href="#fn-%d">%d</a></sup>`,
		fnrefID(n.Index, n.RefIndex), n.Index, n.Index)
	return ast.WalkContinue, nil
}

func (r *epubFootnoteRenderer) renderFootnoteBacklink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.FootnoteBacklink)
	// Backlinks target the first inline reference of the note.
	fmt.Fprintf(w, `<a class="backref" href="#fnref-%d" role="doc-backlink">&#8617;</a>`, n.Index)
	return ast.WalkContinue, nil
}

func (r *epubFootnoteRenderer) renderFootnoteList(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		w.WriteString(`<hr class="fn-sep" /><section class="footnotes" epub:type="footnotes">` + "\n")
	} else {
		w.WriteString("</section>\n")
	}
	return ast.WalkContinue, nil
}

func (r *epubFootnoteRenderer) renderFootnote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*extast.Footnote)
	if entering {
		fmt.Fprintf(w, `<aside epub:type="footnote" role="doc-footnote" id="fn-%d">`+"\n", n.Index)
	} else {
		w.WriteString("</aside>\n")
	}
	return ast.WalkContinue, nil
}

// Esc escapes text for XML content and attribute values.
func Esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return strings.ReplaceAll(b.String(), `"`, "&quot;")
}
