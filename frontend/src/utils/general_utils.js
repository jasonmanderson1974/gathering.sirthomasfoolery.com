/*
  General utils
*/

import { eventTypes, serverURL } from "@/constants"
import { dateToDowDate, dateToTimeNum } from "./date_utils"
import Color from "color"

var timeoutId
/** Calls callback() on long press */
export const onLongPress = (element, callback, capture = false) => {
  element.addEventListener(
    "touchstart",
    function (e) {
      timeoutId = setTimeout(function () {
        timeoutId = null
        e.stopPropagation()
        callback(e.target)
      }, 500)
    },
    capture
  )

  element.addEventListener(
    "contextmenu",
    function (e) {
      e.preventDefault()
    },
    capture
  )

  element.addEventListener(
    "touchend",
    function () {
      if (timeoutId) clearTimeout(timeoutId)
    },
    capture
  )

  element.addEventListener(
    "touchmove",
    function () {
      if (timeoutId) clearTimeout(timeoutId)
    },
    capture
  )
}

/** Returns whether the given value is between lower and upper */
export const isBetween = (value, lower, upper, inclusive = true) => {
  if (inclusive) {
    return value >= lower && value <= upper
  } else {
    return value > lower && value < upper
  }
}

/** Clamps the given value between the given ranges */
export const clamp = (value, lower, upper) => {
  if (value < lower) return lower
  if (value > upper) return upper
  return value
}

/*
 * Viewport helpers. Every one of them takes `$vuetify` rather than reaching for
 * it, and every read of `vuetify.breakpoint` in the app goes through this
 * block — deliberately, because Vuetify 3 renames `$vuetify.breakpoint` to
 * `$vuetify.display` (with `thresholds` becoming lower bounds rather than
 * upper). Keeping the reads here makes that rename one file instead of a sweep
 * through views and components.
 */

export const isPhone = (vuetify) => {
  return vuetify.breakpoint.name === "xs"
}

export const isIOS = () => {
  return /iPad|iPhone|iPod/.test(navigator.userAgent)
}

export const br = (vuetify, breakpoint) => {
  return vuetify.breakpoint.name === breakpoint
}

/**
 * Viewport width in px, for the handful of places that compare against a
 * Tailwind breakpoint rather than a Vuetify one — the two scales don't line up.
 */
export const viewportWidth = (vuetify) => {
  return vuetify.breakpoint.width
}

/** True from Vuetify's own `lg` breakpoint upward. */
export const isLgAndUp = (vuetify) => {
  return vuetify.breakpoint.lgAndUp
}

/** convert base64 to raw binary data held in a string */
export const dataURItoBlob = (dataURI) => {
  // doesn't handle URLEncoded DataURIs - see SO answer #6850276 for code that does this
  var byteString = atob(dataURI.split(",")[1])

  // separate out the mime component
  var mimeString = dataURI.split(",")[0].split(":")[1].split(";")[0]

  // write the bytes of the string to an ArrayBuffer
  var ab = new ArrayBuffer(byteString.length)
  var ia = new Uint8Array(ab)
  for (var i = 0; i < byteString.length; i++) {
    ia[i] = byteString.charCodeAt(i)
  }

  return new Blob([ab], { type: mimeString })
}

/** Reformats the given event object to the format we want */
export const processEvent = (event) => {
  let startDate = event.dates[0]
  if (event.type === eventTypes.DOW) {
    startDate = dateToDowDate(event.dates, startDate, 0, true)
  }

  event.startTime = dateToTimeNum(new Date(startDate), true)
  event.endTime = (event.startTime + event.duration) % 24
}

/** Checks whether email is a valid email */
export const validateEmail = (email) => {
  return String(email)
    .toLowerCase()
    .match(
      /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|.(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/
    )
}

/** Generates a group enabled calendar payload */
export const generateEnabledCalendarsPayload = (calendarAccounts) => {
  const payload = {}

  payload.guest = false
  payload.useCalendarAvailability = true
  payload.enabledCalendars = {}

  /** Determine which sub calendars are enabled */
  for (const email in calendarAccounts) {
    if (calendarAccounts[email].enabled) {
      payload.enabledCalendars[email] = []
      for (const subCalendarId in calendarAccounts[email].subCalendars) {
        if (calendarAccounts[email].subCalendars[subCalendarId].enabled) {
          payload.enabledCalendars[email].push(subCalendarId)
        }
      }
    }
  }

  return payload
}

/** Returns whether touch is enabled on the device */
export const isTouchEnabled = () => {
  return (
    "ontouchstart" in window ||
    navigator.maxTouchPoints > 0 ||
    navigator.msMaxTouchPoints > 0
  )
}

/** Returns whether the element is in the viewport */
export const isElementInViewport = (
  el,
  { topOffset = 0, leftOffset = 0, rightOffset = 0, bottomOffset = 0 }
) => {
  var rect = el.getBoundingClientRect()

  return (
    rect.top >= topOffset &&
    rect.left >= leftOffset &&
    rect.bottom <=
      (window.innerHeight || document.documentElement.clientHeight) +
        bottomOffset &&
    rect.right <=
      (window.innerWidth || document.documentElement.clientWidth) + rightOffset
  )
}

/** Converts hex with transparency to equivalent hex without transparency (on white background) */
export const removeTransparencyFromHex = (hexColor) => {
  const color = Color(hexColor)

  // Y=255 - P*(255-X) : https://graphicdesign.stackexchange.com/questions/113007/how-to-determine-the-equivalent-opaque-rgb-color-for-a-given-partially-transpare
  const red = 255 - color.alpha() * (255 - color.red())
  const green = 255 - color.alpha() * (255 - color.green())
  const blue = 255 - color.alpha() * (255 - color.blue())

  const newColor = Color.rgb(red, green, blue)
  return newColor.hex()
}

/**
 * Returns whether a color is light or dark
 * Source: https://awik.io/determine-color-bright-dark-using-javascript/
 */
export const lightOrDark = (color) => {
  // Variables for red, green, blue values
  var r, g, b, hsp

  // Check the format of the color, HEX or RGB?
  if (color.match(/^rgb/)) {
    // If RGB --> store the red, green, blue values in separate variables
    color = color.match(
      /^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*(\d+(?:\.\d+)?))?\)$/
    )

    r = color[1]
    g = color[2]
    b = color[3]
  } else {
    // If hex --> Convert it to RGB: http://gist.github.com/983661
    color = +("0x" + color.slice(1).replace(color.length < 5 && /./g, "$&$&"))

    r = color >> 16
    g = (color >> 8) & 255
    b = color & 255
  }

  // HSP (Highly Sensitive Poo) equation from http://alienryderflex.com/hsp.html
  hsp = Math.sqrt(0.299 * (r * r) + 0.587 * (g * g) + 0.114 * (b * b))

  // Using the HSP value, determine whether the color is light or dark
  if (hsp > 127.5) {
    return "light"
  } else {
    return "dark"
  }
}

/**
 * The name to show for a user: their nickname when they have one, otherwise
 * "First Last".
 *
 * Mirrors User.DisplayName() on the server — keep the two in step. Prefer this
 * over concatenating the name fields anywhere a name is rendered, so setting a
 * nickname takes effect everywhere at once.
 *
 * Returns "" for a user with no name at all; callers that need a fallback (a
 * stored snapshot, an email, "Guest") supply their own, since the sensible
 * answer differs by surface.
 */
export const displayName = (user) => {
  if (!user) return ""
  const nickname = (user.nickname ?? "").trim()
  if (nickname) return nickname
  return `${(user.firstName ?? "").trim()} ${(
    user.lastName ?? ""
  ).trim()}`.trim()
}

/**
 * "Nickname (First Last)" for The Roll and the Fellowship directory — the two
 * places that have to stay legible to an admin looking for a specific person.
 * Falls back to the plain name when there is no nickname.
 */
export const rollDisplayName = (member) => {
  if (!member) return ""
  const nickname = (member.nickname ?? "").trim()
  const realName = `${(member.firstName ?? "").trim()} ${(
    member.lastName ?? ""
  ).trim()}`.trim()
  if (!nickname) return realName
  if (!realName) return nickname
  return `${nickname} (${realName})`
}

/**
 * Where to fetch a user's photo from, or "" when they have none and the caller
 * should fall back to a monogram.
 *
 * An uploaded avatar wins over the Google `picture` URL: it is the one the
 * member chose, and `picture` is whatever their Google account happened to have
 * when they last signed in.
 *
 * `avatarUpdatedAt` doubles as the has-a-photo flag and the cache-buster. The
 * server serves the image with `Cache-Control: immutable`, so a re-upload is
 * only ever seen because this value — and therefore the URL — changes. It is an
 * RFC 3339 string, not a number (that is how the API serializes every
 * timestamp), which is why it needs encoding.
 */
export const avatarUrl = (user) => {
  if (!user) return ""
  // `userId` wins over `_id`, and the order matters. The roll and the
  // directory render allowlist entries, whose `_id` is the invitation row —
  // not the person. Taking `_id` first there asks for the avatar of an id that
  // has no account and gets a 404 and a broken image. Wherever both appear,
  // `userId` is the one that means "the account this belongs to". A plain user
  // document has no `userId` and falls through to `_id`, and a Google contact
  // has neither, so the id is checked rather than assumed.
  const id = user.userId ?? user._id
  if (user.avatarUpdatedAt && id) {
    return `${serverURL}/users/${id}/avatar?v=${encodeURIComponent(
      user.avatarUpdatedAt
    )}`
  }
  return user.picture ?? ""
}

/**
 * Whether a URL from avatarUrl points at our own (authenticated) avatar route
 * rather than at the Google `picture` fallback.
 *
 * The serving route requires a session, so its <img> has to send the cookie. In
 * production that is automatic — `serverURL` is the relative "/api", making the
 * request same-origin. Under `npm run serve` the SPA is on :8080 and the API on
 * :3002, where a plain <img> sends no credentials and the image 401s; those
 * need `crossorigin="use-credentials"`.
 *
 * The attribute cannot simply go on every avatar: Google's CDN answers no
 * credentialed CORS request, so putting it on a `picture` URL would break the
 * fallback for every member who has not uploaded a photo.
 */
export const isOwnAvatarUrl = (url) =>
  typeof url === "string" && url.startsWith(`${serverURL}/users/`)

/** The square the cropper exports and the server stores. */
export const AVATAR_EXPORT_PX = 256

/**
 * Why this source image can't make a usable avatar, or null if it can.
 *
 * The crop is exported at AVATAR_EXPORT_PX square, and neither the cropper nor
 * the server ever enlarges beyond what it is given — `getCroppedCanvas` scales
 * the selection up to fill the export, and the server's box filter only ever
 * shrinks. So a source whose SHORTEST side is under the export size can only
 * produce a blown-up, soft square, and the shortest side is what bounds it: the
 * crop is square, so a 2000x10 strip yields a 10px selection.
 *
 * H9: this is the only guard on the way in. The save-time "could not be saved"
 * branch was assumed to be catching these — it isn't; it never fires. A tiny or
 * strip-shaped image went all the way through and was stored as a smear.
 *
 * Returns a message rather than a boolean so it can name the actual size: the
 * complaint that sent people in circles was one that didn't say what was wrong.
 */
export const avatarSourceError = (width, height) => {
  if (
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return "That image could not be read."
  }

  const shortest = Math.min(width, height)
  if (shortest < AVATAR_EXPORT_PX) {
    return (
      `That image is too small — it's ${width}x${height}, and a photo needs to be ` +
      `at least ${AVATAR_EXPORT_PX} pixels on its shortest side. A smaller one can ` +
      `only be blown up, and comes out blurry.`
    )
  }

  return null
}

/**
 * The initials to show when there is no photo.
 *
 * A nickname is one word, so it gets one initial; a real name still gets two.
 * Either way the monogram starts with the same letter as the name displayed
 * beside it, which is the point — see displayName.
 *
 * Falls back to the email for accounts with no name at all, and to "?" for a
 * user with nothing to go on.
 */
export const monogram = (user) => {
  if (!user) return ""
  const nickname = (user.nickname ?? "").trim()
  if (nickname) return nickname.charAt(0).toUpperCase()

  const first = (user.firstName ?? "").trim()
  const last = (user.lastName ?? "").trim()
  if (first || last) {
    return `${first.charAt(0)}${last.charAt(0)}`.toUpperCase()
  }

  const email = (user.email ?? "").trim()
  return email ? email.charAt(0).toUpperCase() : "?"
}

/**
 * A stand-in user built from a stored display-name snapshot, for the rows whose
 * account did not resolve — a guest, or an account since deleted. The server
 * attaches the real account wherever it can (comment.author, rsvp.user); this
 * is the fallback for everywhere it cannot.
 *
 * A snapshot is a finished display name, not structured fields, so it is split
 * back apart: a two-word name yields the same two initials the account itself
 * would have produced. Anything the caller passes that isn't a string yields an
 * empty pair, which monogram renders as "?".
 */
export const userFromDisplayName = (name) => {
  const parts = (typeof name === "string" ? name : "").trim().split(/\s+/)
  return {
    firstName: parts[0] ?? "",
    lastName: parts.length > 1 ? parts[parts.length - 1] : "",
  }
}

/**
 * A Google Maps search link for a free-text venue or address.
 *
 * Locations are stored as the text the user typed or picked — no place id, no
 * coordinates — so the link is a search rather than a pin. Shared by the
 * event's own location and by the items on a location list, which is why it
 * lives here instead of in either component.
 */
export const mapsSearchUrl = (location) =>
  `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(
    location ?? ""
  )}`

export const prefersStartOnMonday = () => {
  let defaultStartOnMonday = false
  try {
    defaultStartOnMonday = weekStartLocale(navigator.language) === "mon"
  } catch {
    defaultStartOnMonday = false
  }
  return localStorage["startCalendarOnMonday"] == undefined
    ? defaultStartOnMonday
    : localStorage["startCalendarOnMonday"] == "true"
}

/** Source: https://stackoverflow.com/questions/53382465/how-can-i-determine-if-week-starts-on-monday-or-sunday-based-on-locale-in-pure-j */
function weekStart(region, language) {
  const regionSat = "AEAFBHDJDZEGIQIRJOKWLYOMQASDSY".match(/../g)
  const regionSun =
    "AGARASAUBDBRBSBTBWBZCACNCODMDOETGTGUHKHNIDILINJMJPKEKHKRLAMHMMMOMTMXMZNINPPAPEPHPKPRPTPYSASGSVTHTTTWUMUSVEVIWSYEZAZW".match(
      /../g
    )
  const languageSat = ["ar", "arq", "arz", "fa"]
  const languageSun =
    "amasbndzengnguhehiidjajvkmknkolomhmlmrmtmyneomorpapssdsmsnsutatethtnurzhzu".match(
      /../g
    )

  return region
    ? regionSun.includes(region)
      ? "sun"
      : regionSat.includes(region)
      ? "sat"
      : "mon"
    : languageSun.includes(language)
    ? "sun"
    : languageSat.includes(language)
    ? "sat"
    : "mon"
}
function weekStartLocale(locale) {
  const parts = locale.match(
    /^([a-z]{2,3})(?:-([a-z]{3})(?=$|-))?(?:-([a-z]{4})(?=$|-))?(?:-([a-z]{2}|\d{3})(?=$|-))?/i
  )
  return weekStart(parts[4], parts[1])
}
