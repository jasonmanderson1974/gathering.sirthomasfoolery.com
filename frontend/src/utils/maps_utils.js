/**
 * Google Places (New) address lookup for the event location field.
 *
 * Everything here is optional: with no VUE_APP_GOOGLE_MAPS_API_KEY configured
 * (or if Google fails to load) every function degrades to "no suggestions",
 * and the location inputs stay usable as plain free-text fields. Callers must
 * never depend on a suggestion being available.
 *
 * Uses the Place Autocomplete *Data* API rather than the prebuilt widget:
 * `google.maps.places.Autocomplete` has not been available to new Maps
 * customers since 2025-03-01, and fetching raw predictions lets us render them
 * in our own themed input instead of Google's unstyleable element.
 */

const API_KEY = process.env.VUE_APP_GOOGLE_MAPS_API_KEY

/** Whether address lookup is configured at all */
export const isPlacesEnabled = () => !!API_KEY

let placesPromise = null

/**
 * Lazily loads the Maps JS bootstrap and the places library, once per page.
 * Resolves to the places library, or null when unavailable for any reason.
 */
export const loadPlaces = () => {
  if (!API_KEY) return Promise.resolve(null)
  if (placesPromise) return placesPromise

  placesPromise = new Promise((resolve) => {
    const done = () => {
      window.google.maps
        .importLibrary("places")
        .then(resolve)
        .catch((err) => {
          console.error("Places library failed to load", err)
          resolve(null)
        })
    }

    if (window.google?.maps?.importLibrary) {
      done()
      return
    }

    const script = document.createElement("script")
    script.async = true
    script.src = `https://maps.googleapis.com/maps/api/js?key=${encodeURIComponent(
      API_KEY
    )}&libraries=places&loading=async&callback=__initGoogleMaps`
    window.__initGoogleMaps = done
    script.onerror = () => {
      console.error("Google Maps failed to load")
      resolve(null)
    }
    document.head.appendChild(script)
  })

  return placesPromise
}

/**
 * Starts a new autocomplete session. Google bills a session (keystrokes +
 * one place lookup) more cheaply than the individual requests, so callers
 * should hold one token per "user is picking a place" interaction and drop it
 * once something is chosen.
 */
export const newSessionToken = async () => {
  const places = await loadPlaces()
  if (!places) return null
  try {
    return new places.AutocompleteSessionToken()
  } catch {
    return null
  }
}

/**
 * Returns address suggestions for what the user has typed so far.
 * Always resolves to an array — empty when lookup is unavailable or errors.
 *
 * Each suggestion is `{ text, placeId }`, where `text` is what we store: the
 * location field is a plain string, so a picked place and typed-in text are
 * indistinguishable downstream (ICS, Chronicle, reminder emails).
 */
export const fetchPlaceSuggestions = async (input, sessionToken = null) => {
  const query = (input ?? "").trim()
  if (query.length < 3) return []

  const places = await loadPlaces()
  if (!places) return []

  try {
    const { suggestions } =
      await places.AutocompleteSuggestion.fetchAutocompleteSuggestions({
        input: query,
        ...(sessionToken ? { sessionToken } : {}),
      })

    return (suggestions ?? [])
      .map((s) => s.placePrediction)
      .filter(Boolean)
      .map((p) => ({
        text: p.text?.toString() ?? "",
        placeId: p.placeId ?? "",
      }))
      .filter((s) => s.text.length > 0)
  } catch (err) {
    // A bad key, a disabled API or an offline browser all land here. Lookup is
    // a convenience, so swallow it and let the user type the address instead.
    console.error("Place lookup failed", err)
    return []
  }
}
