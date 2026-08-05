import { describe, it, expect } from "vitest"
import {
  TOOLBAR_ACTIONS,
  NOTE_MAX_LENGTH,
  applyToolbarAction,
  isNoteDirty,
  describeSaveState,
} from "./markdownEditor"

/** Shorthand: apply an action to `text` with the caret/selection at [a, b]. */
const at = (action, text, a, b = a) =>
  applyToolbarAction(action, {
    text,
    selectionStart: a,
    selectionEnd: b,
  })

/** The text with the resulting selection marked, so failures read clearly. */
const marked = (r) =>
  r.text.slice(0, r.selectionStart) +
  "|" +
  r.text.slice(r.selectionStart, r.selectionEnd) +
  "|" +
  r.text.slice(r.selectionEnd)

describe("inline wrapping", () => {
  it("wraps a selection and leaves the TEXT selected, not the markers", () => {
    // The selection landing inside the markers is what makes the button a
    // toggle — press it again and the un-wrap case matches.
    expect(marked(at("bold", "a bold b", 2, 6))).toBe("a **|bold|** b")
  })

  it("un-wraps when the markers sit outside the selection", () => {
    expect(marked(at("bold", "a **bold** b", 4, 8))).toBe("a |bold| b")
  })

  it("un-wraps when the markers sit inside the selection", () => {
    expect(marked(at("bold", "a **bold** b", 2, 10))).toBe("a |bold| b")
  })

  it("puts the caret between the markers when nothing is selected", () => {
    expect(marked(at("bold", "ab", 1))).toBe("a**||**b")
  })

  it("handles an empty document", () => {
    expect(marked(at("bold", "", 0))).toBe("**||**")
  })

  it("uses one asterisk for italic and a backtick for code", () => {
    expect(marked(at("italic", "word", 0, 4))).toBe("*|word|*")
    expect(marked(at("code", "word", 0, 4))).toBe("`|word|`")
  })

  it("strips exactly one marker's worth when un-wrapping", () => {
    // Bold-and-italic text is "***x***"; italicising "**x**" removes the one
    // asterisk on each side that italic owns, and leaves the bold alone.
    expect(at("italic", "**x**", 2, 3).text).toBe("*x*")
  })
})

describe("line prefixes", () => {
  it("prefixes the line the caret is on, mid-word", () => {
    expect(at("bullet", "milk", 2).text).toBe("- milk")
  })

  it("prefixes every line the selection touches", () => {
    expect(at("bullet", "a\nb\nc", 0, 5).text).toBe("- a\n- b\n- c")
  })

  it("expands a partial selection out to whole lines", () => {
    // Selection covers only the middle of the first and last lines.
    expect(at("quote", "abc\ndef", 1, 5).text).toBe("> abc\n> def")
  })

  it("toggles off when every line already carries the prefix", () => {
    expect(at("bullet", "- a\n- b", 0, 7).text).toBe("a\nb")
  })

  it("adds rather than removes when only some lines carry it", () => {
    expect(at("bullet", "- a\nb", 0, 5).text).toBe("- a\n- b")
  })

  it("numbers a block from 1 and renumbers on re-apply", () => {
    expect(at("numbered", "a\nb\nc", 0, 5).text).toBe("1. a\n2. b\n3. c")
    // An existing, wrongly-numbered block is normalised rather than doubled.
    expect(at("numbered", "4. a\n9. b", 0, 9).text).toBe("a\nb")
  })

  it("leaves blank lines alone inside a block", () => {
    expect(at("bullet", "a\n\nb", 0, 4).text).toBe("- a\n\n- b")
  })

  it("prefixes an empty line, since that is how a list is started", () => {
    expect(at("bullet", "", 0).text).toBe("- ")
  })

  it("replaces any heading level with h2 rather than cycling", () => {
    expect(at("heading", "#### deep", 0, 9).text).toBe("## deep")
  })

  it("removes a heading only when it is already exactly h2", () => {
    expect(at("heading", "## title", 0, 8).text).toBe("title")
  })

  it("selects the whole transformed block", () => {
    expect(marked(at("bullet", "a\nb", 0, 3))).toBe("|- a\n- b|")
  })

  it("does not disturb text outside the block", () => {
    expect(at("bullet", "keep\ntarget\nkeep", 5, 5).text).toBe(
      "keep\n- target\nkeep"
    )
  })
})

describe("link", () => {
  it("selects the url when there is text to link", () => {
    expect(marked(at("link", "click here", 0, 10))).toBe("[click here](|url|)")
  })

  it("selects the label when there is nothing selected", () => {
    // A bare [](url) renders as nothing, so the label is the field more likely
    // to be left unfilled by mistake.
    expect(marked(at("link", "", 0))).toBe("[|text|](url)")
  })

  it("treats a selected URL as the destination", () => {
    expect(marked(at("link", "https://e.com", 0, 13))).toBe(
      "[|text|](https://e.com)"
    )
  })
})

describe("rule", () => {
  it("opens a line when the caret is mid-text", () => {
    expect(at("rule", "a", 1).text).toBe("a\n---\n")
  })

  it("does not add a blank line at the start of a line", () => {
    expect(at("rule", "", 0).text).toBe("---\n")
    expect(at("rule", "a\n", 2).text).toBe("a\n---\n")
  })

  it("leaves the caret after the rule", () => {
    const r = at("rule", "", 0)
    expect(r.selectionStart).toBe(r.text.length)
    expect(r.selectionStart).toBe(r.selectionEnd)
  })
})

describe("robustness", () => {
  it("returns the selection untouched for an unknown action", () => {
    expect(at("nonsense", "keep me", 1, 3)).toEqual({
      text: "keep me",
      selectionStart: 1,
      selectionEnd: 3,
    })
  })

  it("survives a missing selection object", () => {
    expect(applyToolbarAction("bold", undefined).text).toBe("****")
  })

  it("clamps a stale caret past the end of the text", () => {
    // A selection read before the text shrank must not slice out of bounds.
    const r = at("bold", "ab", 99, 99)
    expect(r.text).toBe("ab****")
  })

  it("every toolbar action is implemented", () => {
    for (const action of TOOLBAR_ACTIONS) {
      const r = at(action.id, "sample text", 0, 6)
      expect(r.text, `action ${action.id} did nothing`).not.toBe("sample text")
    }
  })

  it("mirrors the server's cap", () => {
    expect(NOTE_MAX_LENGTH).toBe(20000)
  })
})

describe("isNoteDirty", () => {
  it("is clean when a change is typed and undone", () => {
    expect(isNoteDirty("abc", "abc")).toBe(false)
  })

  it("treats null and empty as the same nothing", () => {
    expect(isNoteDirty(null, "")).toBe(false)
    expect(isNoteDirty("", undefined)).toBe(false)
  })

  it("notices a real difference, whitespace included", () => {
    expect(isNoteDirty("abc ", "abc")).toBe(true)
  })
})

describe("describeSaveState", () => {
  it("puts a failure above everything else", () => {
    expect(
      describeSaveState({ dirty: true, saving: true, failed: true }).tone
    ).toBe("error")
  })

  it("reports saving before dirty", () => {
    expect(describeSaveState({ dirty: true, saving: true }).text).toBe("Saving…")
  })

  it("warns about unsaved changes", () => {
    expect(describeSaveState({ dirty: true }).tone).toBe("brass")
  })

  it("shows the time of the last save when settled", () => {
    expect(describeSaveState({ savedAtLabel: "12:04" }).text).toBe("Saved 12:04")
  })

  it("says nothing about a note that has never been saved", () => {
    expect(describeSaveState({}).text).toBe("")
  })
})
