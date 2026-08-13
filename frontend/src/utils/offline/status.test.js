import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import {
  __resetStatusForTests,
  isOffline,
  noteNetworkFailure,
  noteNetworkSuccess,
  onOfflineChange,
} from "./status"

beforeEach(() => {
  __resetStatusForTests()
  vi.stubGlobal("navigator", { onLine: true })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("isOffline", () => {
  it("is false when the device is online and nothing has failed", () => {
    expect(isOffline()).toBe(false)
  })

  it("follows navigator.onLine when the device says it is offline", () => {
    vi.stubGlobal("navigator", { onLine: false })
    expect(isOffline()).toBe(true)
  })

  // The case navigator.onLine gets wrong, and the reason this module exists: a
  // phone on a captive portal has an interface up and reports online, but
  // nothing reaches our server.
  it("reports offline when requests fail despite navigator saying online", () => {
    noteNetworkFailure()
    expect(isOffline()).toBe(true)
  })

  it("clears once a request gets through again", () => {
    noteNetworkFailure()
    noteNetworkSuccess()
    expect(isOffline()).toBe(false)
  })

  // A successful request cannot argue with a device that has no interface up.
  it("still reports offline if navigator says so, whatever the last request did", () => {
    noteNetworkSuccess()
    vi.stubGlobal("navigator", { onLine: false })
    expect(isOffline()).toBe(true)
  })
})

describe("subscribers", () => {
  it("notifies on a transition and passes the new state", () => {
    const seen = []
    onOfflineChange((offline) => seen.push(offline))
    noteNetworkFailure()
    noteNetworkSuccess()
    expect(seen).toEqual([true, false])
  })

  it("does not notify when the state has not changed", () => {
    const fn = vi.fn()
    onOfflineChange(fn)
    noteNetworkFailure()
    noteNetworkFailure()
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it("stops notifying after unsubscribe", () => {
    const fn = vi.fn()
    onOfflineChange(fn)()
    noteNetworkFailure()
    expect(fn).not.toHaveBeenCalled()
  })

  // One bad subscriber must not stop the others hearing about it.
  it("keeps notifying the rest when one subscriber throws", () => {
    const fn = vi.fn()
    onOfflineChange(() => {
      throw new Error("boom")
    })
    onOfflineChange(fn)
    expect(() => noteNetworkFailure()).not.toThrow()
    expect(fn).toHaveBeenCalledWith(true)
  })
})
