/**
 * Pure helpers for the shared lists (F16), kept out of EventLists.vue so they
 * are testable: the vitest env is node with no jsdom, so nothing inside a .vue
 * file can be reached. Same split commentThreads.js uses for the discussion.
 */

/**
 * How many levels of nesting a list allows: a top-level entry, a child and a
 * grandchild. Mirrors maxListItemDepth in routes/event_lists.go — the server
 * is what actually enforces it; this is so the UI doesn't offer what would be
 * refused.
 */
export const MAX_ITEM_DEPTH = 3

/**
 * Flatten a list's items into the rows to render, depth-first, parents before
 * their children.
 *
 * Items are stored flat with a parentId rather than nested, so this is where
 * the tree is actually built. Two shapes need care:
 *
 * - An item whose parent isn't there — its parent was deleted between someone
 *   else's read and write — renders at the top level rather than vanishing.
 *   Dropping it would hide an entry someone typed.
 * - A cycle can't be produced by the app, but a hand-edited document could
 *   hold one, and rendering must not hang. Every item is emitted at most once.
 *
 * @param {Array} items the list's items, in insertion order
 * @param {Array} collapsedIds ids of items whose children are hidden
 * @returns {Array<{item, depth, hasChildren, collapsed}>}
 */
export const flattenListItems = (items, collapsedIds = []) => {
  const all = items ?? []
  const ids = new Set(all.map((item) => item._id))

  // Group by parent, treating an unresolvable parent as none. Insertion order
  // is preserved within each group, which is the order the API returns.
  const childrenOf = new Map()
  for (const item of all) {
    const parentId =
      item.parentId && ids.has(item.parentId) ? item.parentId : null
    if (!childrenOf.has(parentId)) childrenOf.set(parentId, [])
    childrenOf.get(parentId).push(item)
  }

  const collapsed = new Set(collapsedIds ?? [])
  const emitted = new Set()
  const rows = []

  const walk = (parentId, depth) => {
    for (const item of childrenOf.get(parentId) ?? []) {
      if (emitted.has(item._id)) continue
      emitted.add(item._id)

      const hasChildren = (childrenOf.get(item._id) ?? []).length > 0
      const isCollapsed = hasChildren && collapsed.has(item._id)
      rows.push({ item, depth, hasChildren, collapsed: isCollapsed })

      if (hasChildren && !isCollapsed) {
        walk(item._id, depth + 1)
      }
    }
  }
  walk(null, 0)

  return rows
}

/**
 * Whether an entry at this depth may take children. A grandchild cannot, so the
 * "add sub-entry" control is not offered on one.
 */
export const canAddChild = (depth) => depth < MAX_ITEM_DEPTH - 1

/**
 * The line describing a checklist entry's state, or null for one nobody has
 * touched. Unchecking is a change of state too, so it is attributed the same
 * way — an item that was ticked and then cleared should not read as untouched.
 */
export const checkStateLabel = (item) => {
  if (!item?.checkedByName) return null
  return item.checked
    ? `Checked by ${item.checkedByName}`
    : `Unchecked by ${item.checkedByName}`
}
