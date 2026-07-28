package utils

import (
	"fmt"
	"html"
	"strings"
)

// Shared layout for every email the app sends.
//
// Before this existed each sender hand-copied the same table shell and built its
// body with fmt.Sprintf, which meant user-controlled text (an event name, a
// venue, a description) went into the HTML unescaped. Everything here escapes on
// the way in, so a gathering called `<script>…` reads as those characters rather
// than running.
//
// Email clients strip <style> blocks and most of CSS, hence the table layout and
// inline styles: this is the shape that renders in Gmail and Outlook, not the
// shape anyone would choose for a web page.

// Palette — the same leather/brass values the app uses.
const (
	emailPageBg   = "#1c1410"
	emailCardBg   = "#241a13"
	emailBorder   = "#8a7333"
	emailInk      = "#ede4d3" // primary text
	emailInkMuted = "#b8ad97" // secondary text
	emailAccent   = "#e3c578" // dates, times, links
	emailBrass    = "#c9a44c" // eyebrow, primary button
)

// EmailAction is a call-to-action button. The first is rendered filled, and any
// marked Secondary are outlined.
type EmailAction struct {
	Label     string
	URL       string
	Secondary bool
}

// EscapeText makes arbitrary text safe to place in an HTML email body.
func EscapeText(s string) string {
	return html.EscapeString(s)
}

// safeURL returns href if it is a plain http(s) URL, else "". Everything we send
// is built from GetBaseUrl(), so this only ever fires on a bug or a crafted
// value — but an unchecked href is a javascript: link waiting to happen.
func safeURL(href string) string {
	trimmed := strings.TrimSpace(href)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return ""
	}
	return html.EscapeString(trimmed)
}

// EmailParagraph renders a block of secondary body copy.
func EmailParagraph(text string) string {
	return fmt.Sprintf(
		`<div style="font-size:14px;color:%s;line-height:1.6;margin-bottom:24px;">%s</div>`,
		emailInkMuted, html.EscapeString(text),
	)
}

// EmailStrongLine renders an emphasised line — typically the gathering's name.
func EmailStrongLine(text string) string {
	return fmt.Sprintf(
		`<div style="font-size:16px;color:%s;margin-bottom:6px;"><strong>%s</strong></div>`,
		emailInk, html.EscapeString(text),
	)
}

// EmailAccentLine renders a highlighted line — typically a date and time.
func EmailAccentLine(text string) string {
	return fmt.Sprintf(
		`<div style="font-size:14px;color:%s;margin-bottom:24px;">%s</div>`,
		emailAccent, html.EscapeString(text),
	)
}

// EmailLink renders an inline link, optionally prefixed with an emoji marker.
// A non-http(s) href degrades to plain text rather than becoming a live link.
func EmailLink(href, label string) string {
	safe := safeURL(href)
	if safe == "" {
		return html.EscapeString(label)
	}
	return fmt.Sprintf(
		`<a href="%s" style="color:%s;text-decoration:none;">%s</a>`,
		safe, emailAccent, html.EscapeString(label),
	)
}

// EmailRow wraps arbitrary already-safe inline HTML in a body row. Use it to
// compose rows out of the helpers above (e.g. an emoji plus an EmailLink).
func EmailRow(innerHTML string) string {
	return fmt.Sprintf(
		`<div style="font-size:14px;color:%s;margin-bottom:24px;">%s</div>`,
		emailInk, innerHTML,
	)
}

// EmailParagraphHTML is EmailParagraph for copy that carries its own inline
// markup (a <strong>, say). innerHTML is trusted: pass literal copy, or escape
// any interpolated value with EscapeText first.
func EmailParagraphHTML(innerHTML string) string {
	return fmt.Sprintf(
		`<div style="font-size:14px;color:%s;line-height:1.6;margin-bottom:24px;">%s</div>`,
		emailInkMuted, innerHTML,
	)
}

// EmailEmphasis marks text inside a paragraph, in the primary ink.
func EmailEmphasis(text string) string {
	return fmt.Sprintf(`<strong style="color:%s;">%s</strong>`, emailInk, html.EscapeString(text))
}

// EmailCodeBlock renders a one-time code in the bordered monospace box the
// sign-in and email-change emails use.
func EmailCodeBlock(code string) string {
	return fmt.Sprintf(
		`<div style="text-align:center;background-color:#2e2117;border:1px solid %s;border-radius:10px;padding:20px;margin-bottom:24px;">`+
			`<span style="font-size:34px;font-weight:bold;letter-spacing:0.32em;color:%s;font-family:'Courier New',monospace;">%s</span></div>`,
		emailBorder, emailAccent, html.EscapeString(code),
	)
}

// EmailFooterURL renders the "Or visit: <url>" fallback for clients that mangle
// buttons.
func EmailFooterURL(url string) string {
	return fmt.Sprintf(
		`<div style="font-size:12px;color:%s;line-height:1.5;">Or visit: <span style="color:%s;">%s</span></div>`,
		emailInkMuted, emailAccent, html.EscapeString(url),
	)
}

// EmailFootnote renders small print below the actions.
func EmailFootnote(text string) string {
	return fmt.Sprintf(
		`<div style="font-size:12px;color:%s;line-height:1.5;">%s</div>`,
		emailInkMuted, html.EscapeString(text),
	)
}

func renderAction(a EmailAction) string {
	safe := safeURL(a.URL)
	if safe == "" || a.Label == "" {
		return ""
	}
	label := html.EscapeString(a.Label)
	if a.Secondary {
		return fmt.Sprintf(
			`<div style="text-align:center;margin-bottom:16px;"><a href="%s" style="display:inline-block;color:%s;text-decoration:none;font-size:13px;border:1px solid %s;padding:9px 22px;border-radius:8px;">%s</a></div>`,
			safe, emailAccent, emailBorder, label,
		)
	}
	return fmt.Sprintf(
		`<div style="text-align:center;margin-bottom:16px;"><a href="%s" style="display:inline-block;background-color:%s;color:%s;font-weight:bold;text-decoration:none;padding:12px 28px;border-radius:8px;letter-spacing:0.04em;">%s</a></div>`,
		safe, emailBrass, emailPageBg, label,
	)
}

// RenderEmail wraps a heading and a pre-built body in the shared shell.
//
// heading and the action labels are escaped here. bodyHTML is trusted and must
// be composed from the Email* helpers above (which escape their own input) —
// never pass raw user text to it.
func RenderEmail(heading string, bodyHTML string, actions ...EmailAction) string {
	return RenderEmailWithFooter(heading, bodyHTML, "", actions...)
}

// RenderEmailWithFooter is RenderEmail with a block rendered *after* the
// actions. That is where the "Or visit: <url>" fallback belongs — it backs up
// the button, so it has to follow it; putting it in bodyHTML pushes it above.
func RenderEmailWithFooter(heading string, bodyHTML string, footerHTML string, actions ...EmailAction) string {
	var buttons strings.Builder
	for _, a := range actions {
		buttons.WriteString(renderAction(a))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background-color:%s;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:%s;">
    <tr>
      <td align="center" style="padding:40px 16px;">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:440px;background-color:%s;border:1px solid %s;border-radius:14px;">
          <tr>
            <td style="padding:32px 36px;font-family:Georgia,'Times New Roman',serif;color:%s;">
              <div style="font-size:13px;font-weight:bold;letter-spacing:0.16em;color:%s;text-transform:uppercase;">The Fellowship</div>
              <div style="height:1px;background-color:%s;margin:18px 0 24px;"></div>
              <div style="font-size:22px;color:%s;margin-bottom:10px;">%s</div>
              %s
              %s
              %s
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		emailPageBg, emailPageBg, emailCardBg, emailBorder, emailInk,
		emailBrass, emailBorder, emailInk, html.EscapeString(heading),
		bodyHTML, buttons.String(), footerHTML,
	)
}
