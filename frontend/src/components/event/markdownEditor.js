/**
 * The formatting toolbar behind My Notes (F20), as pure functions.
 *
 * Every transform takes and returns a plain `{text, selectionStart,
 * selectionEnd}` and touches no DOM. That is what makes the fiddly part — where
 * the selection lands after a toggle, what happens with no selection at all,
 * which lines a prefix applies to — testable at all, since this project's
 * vitest environment is node with no jsdom and nothing inside a .vue can be
 * covered. Same reasoning that shaped eventLists.js and mentionText.js.
 *
 * PersonalNotes.vue owns the DOM half: reading the live caret off the textarea,
 * and putting it back afterwards.
 */

/**
 * The cap, mirrored from maxPersonalNoteLength in
 * server/routes/personal_notes.go. The server REJECTS an over-long note rather
 * than truncating it — silently eating the tail of four pages someone typed by
 * hand would be the worst failure this feature could have — so the field stops
 * typing at the same number and that rejection stays unreachable from a browser.
 */
export const NOTE_MAX_LENGTH = 20000

/** The toolbar, in the order it is rendered. */
export const TOOLBAR_ACTIONS = [
  { id: "bold", icon: "mdi-format-bold", title: "Bold" },
  { id: "italic", icon: "mdi-format-italic", title: "Italic" },
  { id: "heading", icon: "mdi-format-header-2", title: "Heading" },
  { id: "bullet", icon: "mdi-format-list-bulleted", title: "Bulleted list" },
  { id: "numbered", icon: "mdi-format-list-numbered", title: "Numbered list" },
  { id: "quote", icon: "mdi-format-quote-close", title: "Quote" },
  { id: "code", icon: "mdi-code-tags", title: "Code" },
  { id: "link", icon: "mdi-link-variant", title: "Link" },
  { id: "rule", icon: "mdi-minus", title: "Divider" },
]

/** The wrapping markers, by action. */
const INLINE_MARKERS = {
  bold: "**",
  italic: "*",
  code: "`",
}

/**
 * The line-prefix actions. `match` recognises a line that already carries the
 * prefix — a REGEXP rather than a string compare, so `### Title` counts as a
 * heading and `3. thing` counts as numbered.
 */
const LINE_PREFIXES = {
  heading: { prefix: "## ", match: /^#{1,6} / },
  bullet: { prefix: "- ", match: /^[-*] / },
  numbered: { prefix: "1. ", match: /^\d+\. /, numbered: true },
  quote: { prefix: "> ", match: /^> / },
}

/** Clamp an index into the string, so a stale caret can never slice wrongly. */
const clamp = (n, max) => Math.max(0, Math.min(max, n ?? 0))

/**
 * Wrap or unwrap a selection in `marker`.
 *
 * Three cases, and the toggle-off one has two shapes worth naming: the markers
 * may sit OUTSIDE the selection (you selected the word, pressed bold, then
 * pressed it again) or INSIDE it (you dragged across the markers too). Both
 * must un-bold, or the button stops being a toggle exactly when someone leans
 * on it.
 *
 * With no selection at all it inserts the pair and leaves the caret between
 * them, so you can just carry on typing.
 */
const wrapInline = (selection, marker) => {
  const text = selection.text ?? ""
  const start = clamp(selection.selectionStart, text.length)
  const end = clamp(selection.selectionEnd, text.length)
  const len = marker.length

  if (start === end) {
    return {
      text: text.slice(0, start) + marker + marker + text.slice(start),
      selectionStart: start + len,
      selectionEnd: start + len,
    }
  }

  const selected = text.slice(start, end)

  // Markers inside the selection: strip them from the selected text itself.
  if (
    selected.length >= len * 2 &&
    selected.startsWith(marker) &&
    selected.endsWith(marker)
  ) {
    const inner = selected.slice(len, selected.length - len)
    return {
      text: text.slice(0, start) + inner + text.slice(end),
      selectionStart: start,
      selectionEnd: start + inner.length,
    }
  }

  // Markers just outside the selection: cut them from around it.
  if (
    text.slice(start - len, start) === marker &&
    text.slice(end, end + len) === marker
  ) {
    return {
      text: text.slice(0, start - len) + selected + text.slice(end + len),
      selectionStart: start - len,
      selectionEnd: start - len + selected.length,
    }
  }

  return {
    // The selection ends up over the TEXT, not the markers, so pressing the
    // same button again lands in the un-wrap case above.
    text: text.slice(0, start) + marker + selected + marker + text.slice(end),
    selectionStart: start + len,
    selectionEnd: end + len,
  }
}

/**
 * Add or remove a line prefix across every line the selection touches.
 *
 * The selection is first expanded to whole lines — pressing "bullet" with the
 * caret in the middle of a word means that line, not the half of it to the
 * right. If every non-blank line already carries the prefix the whole block is
 * un-prefixed; otherwise every non-blank line gets it.
 *
 * `heading` is the one three-state case, and is settled deliberately: any
 * existing `#`-level is replaced with `## `, and it is removed only when it is
 * already exactly `## `. No silent cycling down through h1…h6.
 */
const toggleLinePrefix = (selection, { prefix, match, numbered }) => {
  const text = selection.text ?? ""
  const start = clamp(selection.selectionStart, text.length)
  const end = clamp(selection.selectionEnd, text.length)

  const blockStart = text.lastIndexOf("\n", start - 1) + 1
  const foundEnd = text.indexOf("\n", end)
  const blockEnd = foundEnd === -1 ? text.length : foundEnd

  const lines = text.slice(blockStart, blockEnd).split("\n")
  // A wholly blank block is the "caret on an empty line" case: prefix it
  // anyway, since that is how someone starts a list.
  const meaningful = lines.filter((line) => line.trim() !== "")
  const targets = meaningful.length ? meaningful : lines

  const allPrefixed = targets.every((line) => {
    if (!match.test(line)) return false
    // Only an exact `## ` counts as "already a heading" for removal purposes.
    return prefix !== "## " || line.startsWith("## ")
  })

  let n = 0
  const next = lines.map((line) => {
    if (meaningful.length && line.trim() === "") return line
    if (allPrefixed) return line.replace(match, "")
    n += 1
    const bare = line.replace(match, "")
    return numbered ? `${n}. ${bare}` : prefix + bare
  })

  const block = next.join("\n")
  return {
    text: text.slice(0, blockStart) + block + text.slice(blockEnd),
    selectionStart: blockStart,
    selectionEnd: blockStart + block.length,
  }
}

/**
 * Insert a link, leaving selected whatever the writer still has to fill in.
 *
 * With text selected that is the URL; with nothing selected it is the label,
 * because a bare `[](https://…)` renders as nothing at all and is the easier
 * mistake to leave behind. Selected text that already looks like a URL is
 * treated as the destination rather than the label.
 */
const insertLink = (selection) => {
  const text = selection.text ?? ""
  const start = clamp(selection.selectionStart, text.length)
  const end = clamp(selection.selectionEnd, text.length)
  const selected = text.slice(start, end)

  if (selected && /^https?:\/\//.test(selected)) {
    const inserted = `[text](${selected})`
    return {
      text: text.slice(0, start) + inserted + text.slice(end),
      selectionStart: start + 1,
      selectionEnd: start + 1 + "text".length,
    }
  }

  if (selected) {
    const inserted = `[${selected}](url)`
    const urlAt = start + selected.length + 3
    return {
      text: text.slice(0, start) + inserted + text.slice(end),
      selectionStart: urlAt,
      selectionEnd: urlAt + "url".length,
    }
  }

  const inserted = "[text](url)"
  return {
    text: text.slice(0, start) + inserted + text.slice(start),
    selectionStart: start + 1,
    selectionEnd: start + 1 + "text".length,
  }
}

/**
 * Insert a horizontal rule on its own line.
 *
 * The leading newline is omitted at the very start of the document, and at the
 * start of a line, so pressing it twice doesn't leave a widening gap.
 */
const insertRule = (selection) => {
  const text = selection.text ?? ""
  const at = clamp(selection.selectionStart, text.length)
  const atLineStart = at === 0 || text[at - 1] === "\n"
  const inserted = `${atLineStart ? "" : "\n"}---\n`
  const caret = at + inserted.length
  return {
    text: text.slice(0, at) + inserted + text.slice(at),
    selectionStart: caret,
    selectionEnd: caret,
  }
}

/**
 * Apply a toolbar action to a selection.
 *
 * TOTAL: an unrecognised action returns the selection untouched, so a typo in a
 * template can never corrupt somebody's note.
 *
 * @param {string} action one of TOOLBAR_ACTIONS' ids
 * @param {{text: string, selectionStart: number, selectionEnd: number}} selection
 * @returns {{text: string, selectionStart: number, selectionEnd: number}}
 */
export const applyToolbarAction = (action, selection) => {
  const current = {
    text: selection?.text ?? "",
    selectionStart: selection?.selectionStart ?? 0,
    selectionEnd: selection?.selectionEnd ?? 0,
  }

  if (INLINE_MARKERS[action]) {
    return wrapInline(current, INLINE_MARKERS[action])
  }
  if (LINE_PREFIXES[action]) {
    return toggleLinePrefix(current, LINE_PREFIXES[action])
  }
  if (action === "link") return insertLink(current)
  if (action === "rule") return insertRule(current)

  return current
}

/**
 * Whether the draft differs from what is saved.
 *
 * A string comparison rather than a flag set on input, because typing a
 * character and deleting it again must leave the note clean — and a flag cannot
 * know that.
 *
 * COMPARED TRIMMED, because that is the unit the server stores in: setPersonalNote
 * does strings.TrimSpace before writing, so "note\n" comes back as "note". Comparing
 * raw, the draft and the saved copy could never agree about a note ending in
 * whitespace — and since a save that leaves the note dirty schedules another one,
 * the two of them sat there PUTting the same text at each other every 1.5s
 * forever. A trailing newline is not exotic: the divider button ends in one, and
 * so does pressing Enter.
 *
 * Trimmed comparison is also just correct on its own terms. Trailing whitespace is
 * the one edit the server will discard, so it is the one edit there is no point
 * saving.
 */
export const isNoteDirty = (draft, saved) =>
  (draft ?? "").trim() !== (saved ?? "").trim()

/**
 * The one-line status under the editor.
 *
 * `savedAtLabel` arrives pre-formatted so this module needs no date library,
 * and stays a pure string function.
 */
export const describeSaveState = ({ dirty, saving, failed, savedAtLabel }) => {
  if (failed) return { text: "Couldn't save — try again", tone: "error" }
  if (saving) return { text: "Saving…", tone: "dim" }
  if (dirty) return { text: "Unsaved changes", tone: "brass" }
  if (savedAtLabel) return { text: `Saved ${savedAtLabel}`, tone: "dim" }
  return { text: "", tone: "dim" }
}
