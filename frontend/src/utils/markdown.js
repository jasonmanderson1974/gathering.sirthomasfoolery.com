import { marked } from "marked"
import DOMPurify from "dompurify"

/**
 * Markdown → HTML that is safe to hand to `v-html`.
 *
 * Deliberately NOT unit-tested, and the omission is the reason this file holds
 * configuration rather than logic: DOMPurify needs a real `window`, and this
 * project's vitest environment is node with no jsdom (see vitest.config.mjs).
 * Anything here would be untestable, so there is as little here as possible —
 * every decision the notes editor makes lives in
 * components/event/markdownEditor.js, which is pure and covered.
 *
 * Not exported through utils/index.js on purpose. That barrel is `export *` and
 * some forty components import from `@/utils`; routing a DOM-dependent module
 * through it would drag DOMPurify into every one of their import graphs, and
 * into any future test that imports the barrel. Import `@/utils/markdown`
 * directly.
 */

marked.setOptions({
  gfm: true,
  // A single newline is a line break. This is the most consequential choice in
  // the file: nobody typing a personal note means "two blank lines or it
  // doesn't count", and the default would silently run their lines together.
  breaks: true,
})

// Registered ONCE, at module scope. DOMPurify is a singleton and addHook
// appends, so calling this per render would stack a fresh hook on every
// keystroke.
DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (node.tagName === "A" && node.hasAttribute("href")) {
    node.setAttribute("target", "_blank")
    node.setAttribute("rel", "noopener noreferrer")
  }
})

/**
 * The tags a note may render. EXPLICIT rather than DOMPurify's defaults, so
 * widening it is a deliberate edit someone has to make on purpose.
 *
 * `img` is absent by design. A note is private, and an `<img src>` is an
 * outbound request that would tell a third party the note had been opened —
 * the one way a private document could leak the fact of itself. There is no
 * upload path here either way.
 */
const ALLOWED_TAGS = [
  "p",
  "br",
  "hr",
  "strong",
  "em",
  "del",
  "code",
  "pre",
  "blockquote",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "ul",
  "ol",
  "li",
  "a",
  "table",
  "thead",
  "tbody",
  "tr",
  "th",
  "td",
]

// `target` and `rel` are here because the hook above adds them; DOMPurify runs
// the allowlist over what the hook leaves behind, so omitting them would strip
// the very attributes we just set.
const ALLOWED_ATTR = ["href", "title", "target", "rel", "start"]

/**
 * Render markdown to sanitized HTML.
 *
 * `javascript:` and `data:` URLs are refused by DOMPurify's default URI regexp,
 * which is deliberately not overridden.
 */
export const renderMarkdown = (text) =>
  DOMPurify.sanitize(marked.parse(text ?? ""), {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ALLOW_DATA_ATTR: false,
  })
