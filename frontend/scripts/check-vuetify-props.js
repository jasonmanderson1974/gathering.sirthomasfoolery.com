#!/usr/bin/env node
/*
 * Does every prop we bind on a Vuetify component actually exist in Vuetify?
 *
 * WHY THIS EXISTS: Vue 3 passes an unrecognised prop straight through as a DOM
 * attribute and says nothing about it. A Vuetify 2 prop that no longer exists
 * in Vuetify 3 therefore survives lint, the unit suite, the production build
 * and `check:routes` — the element still renders, it just renders wrong. That
 * is exactly how the post-migration review found a `fab fixed` create button
 * rendering square and 1,300px below the fold (TODO3 L2), and five more props
 * silently discarded (L3).
 *
 * The authority for "does this prop exist" is Vuetify's own shipped type
 * declarations, already on disk in node_modules. Nothing else in the pipeline
 * consults them. This is the "it rendered wrong" half of the safety net;
 * `check:routes` is the "it did not render" half.
 *
 * Usage: node scripts/check-vuetify-props.js   (exit 1 on any unknown prop)
 */
const fs = require("fs")
const path = require("path")
const { parse } = require("@vue/compiler-sfc")

const SRC = path.join(__dirname, "..", "src")
const VUETIFY = path.join(__dirname, "..", "node_modules", "vuetify", "lib")

/*
 * Attributes that are legitimately not Vuetify props: they fall through to the
 * DOM and mean something there. Note that for the input components Vuetify
 * routes everything except class/style/id/inert/data-* onto the inner `<input>`
 * (`filterInputAttrs` in vuetify/lib/util/helpers.js), so `maxlength`,
 * `required` and friends do land where they are meant to.
 */
const NATIVE = new Set([
  "class",
  "style",
  "key",
  "ref",
  "id",
  "slot",
  "is",
  "type",
  "role",
  "title",
  "for",
  "form",
  "target",
  "rel",
  "download",
  "required",
  "autocomplete",
  "autocapitalize",
  "autocorrect",
  "spellcheck",
  "inputmode",
  "enterkeyhint",
  "maxlength",
  "minlength",
  "pattern",
  "step",
  "min",
  "max",
  "cols",
  "wrap",
  "onsubmit",
  "contenteditable",
  "draggable",
  "tabindex",
])
const NATIVE_PREFIX = /^(data-|aria-|on[A-Z])/

const walk = (dir, out = []) => {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, out)
    else out.push(full)
  }
  return out
}

const camel = (s) => s.replace(/-(\w)/g, (_, c) => c.toUpperCase())

/*
 * Vuetify declares each component's full, flattened prop list as the `Defaults`
 * constraint of its `makeVXxxProps` factory:
 *
 *   export declare const makeVBtnProps: <Defaults extends {
 *       density?: unknown;
 *       size?: unknown;
 *       ...
 *   } = {}>(defaults?: Defaults | undefined) => { ... }
 *
 * Read the keys at depth 1 of that object literal. Components built by
 * `createSimpleFunctional` (VSpacer, VCardTitle, …) have no factory and take no
 * props of their own; they simply do not appear in the map and are skipped.
 */
const readVuetifyProps = () => {
  const byComponent = new Map()
  for (const file of walk(path.join(VUETIFY, "components"))) {
    if (!file.endsWith(".d.ts")) continue
    const src = fs.readFileSync(file, "utf8")
    const re = /export declare const make(V\w+)Props: <Defaults extends \{/g
    let m
    while ((m = re.exec(src))) {
      const props = new Set()
      let depth = 1
      for (let i = re.lastIndex; i < src.length && depth > 0; i++) {
        const ch = src[i]
        if (ch === "{") depth++
        else if (ch === "}") depth--
        else if (depth === 1 && /[A-Za-z_$]/.test(ch)) {
          const key = /^["']?([A-Za-z0-9_$]+)["']?\??:/.exec(src.slice(i))
          if (key) {
            props.add(key[1])
            i += key[0].length - 1
          }
        }
      }
      byComponent.set(m[1], props)
    }
  }
  return byComponent
}

/** `v-btn-toggle` → `VBtnToggle` */
const tagToComponent = (tag) =>
  tag
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("")

const vuetifyProps = readVuetifyProps()
const findings = []

const visit = (node, file, source) => {
  if (node.type === 1 /* ELEMENT */ && /^v-[a-z-]+$/.test(node.tag)) {
    const known = vuetifyProps.get(tagToComponent(node.tag))
    if (known) {
      for (const prop of node.props) {
        let name
        if (prop.type === 6 /* ATTRIBUTE */) name = prop.name
        else if (prop.type === 7 /* DIRECTIVE */) {
          // Only `:foo` / `v-bind:foo` binds a prop by name. v-model, v-if,
          // v-slot, v-on and `v-bind="$attrs"` do not name one statically.
          if (prop.name !== "bind") continue
          if (!prop.arg || prop.arg.isStatic === false) continue
          name = prop.arg.content
        } else continue

        if (NATIVE.has(name) || NATIVE_PREFIX.test(name)) continue
        if (known.has(camel(name))) continue

        const line = source.slice(0, prop.loc.start.offset).split("\n").length
        findings.push({ file, line, tag: node.tag, name })
      }
    }
  }
  for (const child of node.children ?? []) visit(child, file, source)
}

for (const file of walk(SRC)) {
  if (!file.endsWith(".vue")) continue
  const source = fs.readFileSync(file, "utf8")
  const { descriptor, errors } = parse(source, { filename: file })
  if (errors.length) {
    console.error(`${path.relative(SRC, file)}: ${errors[0].message}`)
    process.exitCode = 1
    continue
  }
  if (descriptor.template?.ast) {
    visit(descriptor.template.ast, path.relative(path.join(SRC, ".."), file), source)
  }
}

console.log(
  `checked ${vuetifyProps.size} Vuetify components against src/**/*.vue`
)

if (findings.length === 0) {
  console.log("no unknown props")
  process.exit(process.exitCode ?? 0)
}

for (const f of findings) {
  console.error(`${f.file}:${f.line}  <${f.tag}> has no prop "${f.name}"`)
}
console.error(
  `\n${findings.length} unknown prop${findings.length === 1 ? "" : "s"} — ` +
    `Vue 3 passes these through as DOM attributes and says nothing.`
)
process.exit(1)
