/*
  Where a write decides to wait.

  One function, wrapped round the service calls that the queue knows how to
  replay. Everything else — creating a gathering, assigning a checklist entry,
  editing event settings — is untouched and still fails offline, which is the
  honest outcome for a write with no reducer behind it.

  Who the viewer is has to come from somewhere, because a reducer writing a
  comment into the cache needs an author to render. It cannot come from the
  store: the store imports the services, so a service importing the store would
  close a cycle. It is pushed in instead, from the one mutation that learns it —
  the same registration idiom `setUnauthorizedHandler` uses.
*/
import { isOffline } from "./status"
import { enqueue } from "./queue"

let viewer = { id: null, name: "" }

/** Called by the store's setAuthUser mutation. */
export const setOfflineViewer = (authUser) => {
  viewer = authUser
    ? {
        id: authUser._id ?? null,
        name:
          authUser.nickname ||
          [authUser.firstName, authUser.lastName].filter(Boolean).join(" ") ||
          authUser.email ||
          "",
      }
    : { id: null, name: "" }
}

export const offlineViewer = () => viewer

/**
 * Sends `request` now, or queues `kind` to be sent later if it can't.
 *
 * Both paths are covered on purpose. `isOffline()` catches the case we already
 * know about; the catch covers the request that was in flight when the signal
 * went, which is the one a check-then-send would drop on the floor.
 *
 * A queued write returns a stand-in result — for a create, an object carrying
 * the temporary id — so a caller awaiting it carries on exactly as it would
 * have. The cache has already been updated by then, so the refetch every call
 * site does next reads the change back.
 */
export const offlineWrite = async (kind, fields, request) => {
  const enriched = () => ({
    ...fields,
    userId: viewer.id,
    authorName: viewer.name,
  })

  if (isOffline()) return enqueue(kind, enriched())

  try {
    return await request()
  } catch (err) {
    if (err?.offline) return enqueue(kind, enriched())
    throw err
  }
}
