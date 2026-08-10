// ESLint config — fully blocking as of TODO2 H4 (2026-07-29). The backlog that
// justified warn-level tolerance is cleared (0 warnings), so the rules below
// inherit `error` from `eslint:recommended` / `plugin:vue/essential`, and the
// lint script runs with `--max-warnings 0` so any *new* warning fails CI too.
// Vue 2.7 project, so the Vue 2 preset (`plugin:vue/essential`) is used, not
// the vue3-* configs.
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
  extends: ["eslint:recommended", "plugin:vue/essential"],
  rules: {
    // Loop conditions like `while (true)` are idiomatic here; only the
    // non-loop form is worth flagging.
    "no-constant-condition": ["error", { checkLoops: false }],
    // View components are intentionally single-word (Home, Group, Friends…) —
    // this project doesn't follow the multi-word convention, so it's pure noise.
    "vue/multi-word-component-names": "off",
  },
};
