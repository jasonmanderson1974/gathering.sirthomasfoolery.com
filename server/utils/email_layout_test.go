package utils

import (
	"strings"
	"testing"
)

// The whole point of this layout is that user-controlled text can't become
// markup, so escaping is what these tests are really about.
const xss = `<script>alert("x")</script>`

func TestRenderEmailEscapesHeading(t *testing.T) {
	out := RenderEmail(xss, "")
	if strings.Contains(out, "<script>") {
		t.Errorf("heading was not escaped:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped heading, got:\n%s", out)
	}
}

func TestRenderEmailShell(t *testing.T) {
	out := RenderEmail("A gathering approaches", EmailParagraph("body"))
	for _, want := range []string{
		"<!DOCTYPE html>",
		"The Fellowship",
		emailPageBg,
		emailCardBg,
		"Georgia",
		"A gathering approaches",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("shell missing %q", want)
		}
	}
}

func TestBodyHelpersEscape(t *testing.T) {
	tests := []struct {
		name string
		got  string
	}{
		{"paragraph", EmailParagraph(xss)},
		{"strong line", EmailStrongLine(xss)},
		{"accent line", EmailAccentLine(xss)},
		{"footer url", EmailFooterURL(xss)},
		{"footnote", EmailFootnote(xss)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.got, "<script>") {
				t.Errorf("not escaped: %s", tt.got)
			}
		})
	}
}

func TestEscapeTextHandlesQuotesAndAmpersands(t *testing.T) {
	got := EscapeText(`Tom & "Jerry" <here>`)
	for _, want := range []string{"&amp;", "&#34;", "&lt;", "&gt;"} {
		if !strings.Contains(got, want) {
			t.Errorf("EscapeText(%q) = %q, missing %q", `Tom & "Jerry" <here>`, got, want)
		}
	}
}

func TestEmailLinkRejectsNonHTTPScheme(t *testing.T) {
	got := EmailLink("javascript:alert(1)", "Click me")
	if strings.Contains(got, "href") {
		t.Errorf("javascript: href should not become a link, got %q", got)
	}
	if !strings.Contains(got, "Click me") {
		t.Errorf("label should survive as plain text, got %q", got)
	}
}

func TestEmailLinkAllowsHTTPAndEscapesLabel(t *testing.T) {
	got := EmailLink("https://example.test/e/abc", xss)
	if !strings.Contains(got, `href="https://example.test/e/abc"`) {
		t.Errorf("expected href, got %q", got)
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("label was not escaped: %q", got)
	}
}

func TestRenderEmailActions(t *testing.T) {
	out := RenderEmail("Heading", "",
		EmailAction{Label: "View the Gathering", URL: "https://example.test/e/abc"},
		EmailAction{Label: "I've already responded", URL: "https://example.test/e/abc/responded", Secondary: true},
	)
	if !strings.Contains(out, "View the Gathering") || !strings.Contains(out, "already responded") {
		t.Errorf("expected both actions, got:\n%s", out)
	}
	// filled primary, outlined secondary
	if !strings.Contains(out, "background-color:"+emailBrass) {
		t.Error("primary action should be filled")
	}
	if !strings.Contains(out, "border:1px solid "+emailBorder+";padding:9px 22px") {
		t.Error("secondary action should be outlined")
	}
}

// The "Or visit:" fallback backs up the button, so it has to render after it.
// Putting it in bodyHTML puts it above, which is how it first shipped.
func TestRenderEmailWithFooterOrdersFooterAfterActions(t *testing.T) {
	out := RenderEmailWithFooter(
		"Heading",
		EmailParagraph("body copy"),
		EmailFooterURL("https://example.test/e/abc"),
		EmailAction{Label: "View the Gathering", URL: "https://example.test/e/abc"},
	)

	body := strings.Index(out, "body copy")
	button := strings.Index(out, "View the Gathering")
	footer := strings.Index(out, "Or visit:")

	if body == -1 || button == -1 || footer == -1 {
		t.Fatalf("missing a section (body=%d button=%d footer=%d)", body, button, footer)
	}
	if body >= button || button >= footer {
		t.Errorf("expected body < button < footer, got body=%d button=%d footer=%d", body, button, footer)
	}
}

func TestEmailCodeBlockEscapes(t *testing.T) {
	got := EmailCodeBlock("482913")
	if !strings.Contains(got, "482913") {
		t.Errorf("code missing from block: %q", got)
	}
	if !strings.Contains(got, "Courier New") {
		t.Error("code block should be monospace")
	}
	if strings.Contains(EmailCodeBlock(xss), "<script>") {
		t.Error("code block did not escape its input")
	}
}

func TestEmailParagraphHTMLKeepsTrustedMarkup(t *testing.T) {
	got := EmailParagraphHTML("sign in with " + EmailEmphasis("this email address"))
	if !strings.Contains(got, "<strong") {
		t.Errorf("trusted inline markup should survive: %q", got)
	}
	if !strings.Contains(got, "this email address") {
		t.Errorf("text missing: %q", got)
	}
}

func TestEmailEmphasisEscapes(t *testing.T) {
	if strings.Contains(EmailEmphasis(xss), "<script>") {
		t.Error("EmailEmphasis did not escape its input")
	}
}

func TestRenderEmailDropsUnusableActions(t *testing.T) {
	out := RenderEmail("Heading", "",
		EmailAction{Label: "Bad scheme", URL: "javascript:alert(1)"},
		EmailAction{Label: "", URL: "https://example.test"},
	)
	if strings.Contains(out, "Bad scheme") {
		t.Errorf("javascript: action should be dropped, got:\n%s", out)
	}
	if strings.Contains(out, `href="https://example.test"`) {
		t.Errorf("label-less action should be dropped, got:\n%s", out)
	}
}
