// ESLint config — fully blocking as of TODO2 H4 (2026-07-29). The backlog that
// justified warn-level tolerance is cleared (0 warnings), so the rules below
// inherit `error` from `eslint:recommended` / `plugin:vue/essential`, and the
// lint script runs with `--max-warnings 0` so any *new* warning fails CI too.
// Vue 3 project (Part K), so the Vue 3 preset is used. `vue3-essential` rather
// than `vue3-recommended` deliberately: this is the same severity tier the Vue
// 2 config sat at, so the migration is not also absorbing a few hundred new
// style-rule violations at the same time as a framework change. Tightening to
// `vue3-recommended` is a follow-up worth doing on a quiet diff.
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
