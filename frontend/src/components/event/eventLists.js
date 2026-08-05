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

  // Group by parent, treating an unresolvable parent as none.
  const childrenOf = new Map()
  for (const item of all) {
    const parentId =
      item.parentId && ids.has(item.parentId) ? item.parentId : null
    if (!childrenOf.has(parentId)) childrenOf.set(parentId, [])
    childrenOf.get(parentId).push(item)
  }

  // Order competes only within a sibling group, so each group is sorted on its
  // own. The sort is STABLE, which is what carries entries written before
  // ordering existed: they all read as 0, tie, and keep the array order the API
  // returned them in — exactly how they looked before drag arrived.
  for (const group of childrenOf.values()) {
    group.sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
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
 * How many entries hang below this one, at any depth.
 *
 * Walks parentId downwards rather than trusting a nesting cap, and is
 * cycle-safe for the same reason flattenListItems is: the app cannot produce a
 * cycle, but a hand-edited document could, and counting must not hang.
 */
export const countDescendants = (items, itemId) => {
  const all = items ?? []
  const childrenOf = new Map()
  for (const item of all) {
    const parentId = item.parentId ?? null
    if (!childrenOf.has(parentId)) childrenOf.set(parentId, [])
    childrenOf.get(parentId).push(item)
  }

  const seen = new Set([itemId])
  let frontier = [itemId]
  let count = 0
  while (frontier.length) {
    const next = []
    for (const id of frontier) {
      for (const child of childrenOf.get(id) ?? []) {
        if (seen.has(child._id)) continue
        seen.add(child._id)
        next.push(child._id)
        count++
      }
    }
    frontier = next
  }
  return count
}

/** Entry text, cut short so a long address can still sit in a dialog title. */
const truncate = (text, limit = 80) => {
  const value = text ?? ""
  return value.length > limit ? `${value.slice(0, limit - 1)}…` : value
}

/**
 * Title and body for deleting a whole list.
 *
 * The count is the point: a list header gives no sign of how much is inside it
 * when collapsed, and deleting one takes every entry with it.
 */
export const describeListDeletion = (list) => {
  const count = (list?.items ?? []).length
  const title = `Delete "${truncate(list?.name)}"?`
  if (count === 0) {
    return { title, body: "This list is empty." }
  }
  return {
    title,
    body: `This removes the list and its ${count} ${
      count === 1 ? "entry" : "entries"
    }.`,
  }
}

/**
 * Title and body for deleting one entry.
 *
 * A body only when there is something to warn about — deleting an entry takes
 * everything nested under it, including sub-entries other people added, and
 * that is the part worth a sentence. A leaf entry gets no body at all.
 */
export const describeItemDeletion = (items, itemId) => {
  const item = (items ?? []).find((i) => i._id === itemId)
  const title = `Delete "${truncate(item?.text)}"?`
  const count = countDescendants(items, itemId)
  if (count === 0) return { title, body: "" }
  return {
    title,
    body: `This also removes its ${count} ${
      count === 1 ? "sub-entry" : "sub-entries"
    }, including any added by other people.`,
  }
}

/**
 * The gap left between adjacent entries' order values. Mirrors
 * listItemOrderStep in routes/event_lists.go — the server stamps this on an
 * add, and the client computes every other position from it, because only the
 * client knows where a drop landed.
 */
export const ORDER_STEP = 1024

/**
 * The order value for an entry dropped between two neighbours.
 *
 * Either bound may be missing: no previous means "above everything", no next
 * means "below everything", neither means the list was empty.
 *
 * The degenerate case is real, if remote. Orders are halved on every drop into
 * the same shrinking gap, and float64 runs out of room after ~60 of them, at
 * which point the midpoint equals one of its own bounds. Returning prev then
 * ties the two entries, which the stable sort renders as the order they already
 * had — a drop that appears not to take, rather than a NaN written to the
 * database.
 */
export const orderBetween = (prevOrder, nextOrder) => {
  const hasPrev = typeof prevOrder === "number" && Number.isFinite(prevOrder)
  const hasNext = typeof nextOrder === "number" && Number.isFinite(nextOrder)

  if (!hasPrev && !hasNext) return ORDER_STEP
  if (!hasPrev) return nextOrder - ORDER_STEP
  if (!hasNext) return prevOrder + ORDER_STEP

  const mid = (prevOrder + nextOrder) / 2
  if (mid <= prevOrder || mid >= nextOrder) return prevOrder
  return mid
}

/**
 * Where a drop actually landed, as the two entries it fell between.
 *
 * This is the whole of the drag logic, kept here rather than in the component
 * because the vitest environment has no jsdom — a .vue file cannot be reached
 * by a test, and this is the part that has to be right.
 *
 * vuedraggable reports positions as indices into the rendered rows. Two shapes
 * come back:
 *
 *   1. The insertion point lies inside the dragged entry's OWN parent's block
 *      of children (same list only) — it is being reordered among its
 *      siblings, so it keeps its parent and competes only with them.
 *   2. Anything else lands at the top level, between the nearest top-level
 *      entries on either side. Dropping halfway inside SOMEONE ELSE'S subtree
 *      therefore resolves to "after that whole subtree": the nearest top-level
 *      row above the insertion point is that subtree's root. The row the
 *      pointer was over and the row the entry ends up next to can differ by a
 *      few positions there, which is the price of never re-parenting.
 *
 * A move never nests an entry under a NEW parent — the server refuses it
 * (`invalid-item-move`), because that is what keeps depth from growing.
 *
 * @param {Object} args
 * @param {Array} args.sourceRows rows of the list the entry came from
 * @param {Array} args.targetRows rows of the list it was dropped on
 * @param {number} args.oldIndex its row index before the drag
 * @param {number} args.newIndex the row index it was released at
 * @param {boolean} args.sameList whether source and target are one list
 * @param {Object} args.draggedItem the entry itself
 * @returns {{parentId: (string|null), prevOrder: (number|null), nextOrder: (number|null)}}
 */
export const resolveDrop = ({
  sourceRows,
  targetRows,
  oldIndex,
  newIndex,
  sameList,
  draggedItem,
}) => {
  // Within one list the entry is lifted out before it is put back, so the
  // index it was released at counts positions in the list WITHOUT it. Across
  // lists the target never held it in the first place.
  const rows = sameList
    ? (sourceRows ?? []).filter((_, i) => i !== oldIndex)
    : targetRows ?? []
  const at = Math.max(0, Math.min(newIndex ?? 0, rows.length))

  const parentId = draggedItem?.parentId ?? null
  if (sameList && parentId) {
    const siblingDrop = resolveSiblingDrop(rows, at, parentId)
    if (siblingDrop) return siblingDrop
  }

  // Top level: the nearest depth-0 rows on either side of the insertion point.
  let prevOrder = null
  for (let i = at - 1; i >= 0; i--) {
    if (rows[i].depth === 0) {
      prevOrder = rows[i].item.order ?? 0
      break
    }
  }
  let nextOrder = null
  for (let i = at; i < rows.length; i++) {
    if (rows[i].depth === 0) {
      nextOrder = rows[i].item.order ?? 0
      break
    }
  }
  return { parentId: null, prevOrder, nextOrder }
}

/**
 * The sibling-reorder case: null unless the insertion point really does sit
 * within the given parent's run of children.
 *
 * The parent's block is its row followed by everything deeper than it, so a
 * grandchild counts as inside the block — but only the parent's DIRECT children
 * are candidates for the entry to land between, since those are the entries it
 * shares an order group with.
 */
const resolveSiblingDrop = (rows, at, parentId) => {
  const parentIndex = rows.findIndex((row) => row.item._id === parentId)
  if (parentIndex === -1) return null

  const parentDepth = rows[parentIndex].depth
  let blockEnd = parentIndex + 1
  while (blockEnd < rows.length && rows[blockEnd].depth > parentDepth) blockEnd++

  // Released above the parent, or past the end of its subtree.
  if (at <= parentIndex || at > blockEnd) return null

  let prevOrder = null
  let nextOrder = null
  for (let i = parentIndex + 1; i < blockEnd; i++) {
    if (rows[i].item.parentId !== parentId) continue
    if (i < at) {
      prevOrder = rows[i].item.order ?? 0
    } else if (nextOrder === null) {
      nextOrder = rows[i].item.order ?? 0
    }
  }
  return { parentId, prevOrder, nextOrder }
}

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

/**
 * Who may create, rename and delete whole lists on an event.
 *
 * Mirrors canManageLists in server/routes/event_lists.go — the server is what
 * actually decides; this exists so the UI doesn't offer what would be refused.
 * Lifted out of EventLists.vue when that component became shared with the
 * private lists panel (F19): the private panel passes this right straight
 * through as `true`, and lifting the rule here is what finally made it
 * testable, since nothing inside a .vue can be.
 *
 * Ownerless legacy events — created before anonymous creation was removed —
 * fall back to member and above.
 */
export const canManageEventLists = ({
  authUser,
  event,
  canManageUsers,
  canInvite,
}) => {
  if (!authUser) return false
  if (canManageUsers) return true
  if (event?.ownerId && event.ownerId !== 0) return authUser._id === event.ownerId
  return !!canInvite
}
