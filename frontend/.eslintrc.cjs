// ESLint config — fully blocking as of TODO2 H4 (2026-07-29). The backlog that
// justified warn-level tolerance is cleared (0 warnings), so the rules below
// inherit `error` from `eslint:recommended` / `plugin:vue/essential`, and the
// lint script runs with `--max-warnings 0` so any *new* warning fails CI too.
// Vue 3 project (Part K), so the Vue 3 preset is used. `vue3-essential` rather
// than `vue3-recommended` deliberately: this is the same severity tier the Vue
// 2 config sat at, so the migration is not also absorbing a few hundred new
// style-rule violations at the same time as a framework change.
//
// SETTLED 2026-08-11 (L13): `vue3-recommended` is NOT worth adopting, and this
// is a decision, not a deferral — don't re-raise it. Measured: 1,941 warnings,
// 0 errors, 1,712 auto-fixable, and every one sampled is formatting that fights
// prettier (`html-indent`, `singleline-html-element-content-newline`,
// `attributes-order`, `max-attributes-per-line`). Zero correctness signal for a
// very large diff.
//
// The signal lives in individual rules, so they are cherry-picked below. Two
// were considered and REJECTED, both for the same underlying reason — an ESLint
// rule sees one file, and this codebase deliberately spreads a component across
// mixins (A5/A11):
//
//   - `vue/no-unused-refs` — 4 hits, all 4 false positives. `name-field`,
//     `emailInput`, `datePicker` and `calendar` are read from mixins.
//   - `vue/no-unused-properties` — 23 hits with the default groups (61 with
//     props+data+computed+methods+setup). Run once in L13 and its one-shot value
//     banked: 19 genuinely dead declarations were deleted. Not enabled, because
//     4 of the 23 were props used by the component's OWN mixin
//     (`interactable`, `showSnackbar`, `animateTimeslotAlways`, `setAuthUser` in
//     ScheduleOverlap), and any future mixin extraction manufactures more.
//     Those false positives are the dangerous kind: they read as dead code, and
//     "fixing" one deletes a working prop — which in Vue 3 then silently becomes
//     a DOM attribute on the root element rather than an error. It also flags
//     `right` on ZigZag, which is a deliberate symmetric API (`left`/`right`)
//     that the component reads as `!left`; removing it would make every call
//     site worse. A rule whose findings need this much judgment belongs in an
//     occasional manual pass, not in a blocking gate.
//
// The vue3 presets also carry the rules that catch removed Vue 2 syntax —
// `vue/no-deprecated-*` — which is precisely what wants flagging here.
//
// The handful of surviving violations are silenced with targeted
// `eslint-disable` comments that state why, next to the code:
//   - OverflowGradient.vue — `scrollContainer` is a raw HTMLElement, so writing
//     `scrollTop` is DOM API, not a prop mutation (false positive).
//   - CalendarAccount.vue / ScheduleOverlap.vue — real prop write-throughs that
//     are load-bearing today; untangling them is tracked as G1 and G2.
module.exports = {
  root: true,
  env: {
    browser: true,
    node: true,
    es2021: true,
  },
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
  },
  extends: ["eslint:recommended", "plugin:vue/vue3-essential"],
  rules: {
    // Loop conditions like `while (true)` are idiomatic here; only the
    // non-loop form is worth flagging.
    "no-constant-condition": ["error", { checkLoops: false }],
    // View components are intentionally single-word (Home, Group, Friends…) —
    // this project doesn't follow the multi-word convention, so it's pure noise.
    "vue/multi-word-component-names": "off",
    // Cherry-picked out of `vue3-recommended` (L7/L13). Not style: in Vue 3 an
    // event that isn't in `emits` stays in `$attrs` and is *additionally* bound
    // as a native listener on the component's root element, so the first
    // `$emit("click")` written without a declaration fires the parent's handler
    // twice, silently. L7 declared all 111 undeclared emits; this keeps it so.
    "vue/require-explicit-emits": "error",
  },
};
