#!/usr/bin/env bash
#
# Are our dependencies carrying known vulnerabilities? Runs both ecosystems and
# exits non-zero only on the findings that can actually reach a user.
#
# WHY THIS EXISTS (TODO3 L10): J12 found six reachable Go vulnerabilities and
# L10 found a pile of npm ones, and both were found the same way — a human
# happened to look. Nothing ran either scanner on a schedule. This does, from
# CI (.github/workflows/dependency-audit.yml) and from here, with the same
# command producing the same verdict in both places.
#
#   scripts/dependency-audit.sh          # both halves
#   scripts/dependency-audit.sh --npm    # just the frontend
#   scripts/dependency-audit.sh --go     # just the server
#
# WHAT BLOCKS, AND WHY IT IS NOT "every advisory"
#
# The frontend's dev toolchain is Vue CLI 5, which is unmaintained upstream and
# is the root of every remaining npm advisory here (webpack-dev-server, babel,
# postcss, serialize-javascript, vue-template-compiler and friends). None of it
# is in `dist`. npm's own remediation for the lot of them is
# `@vue/cli-plugin-babel@3.12.1` — a downgrade to Vue CLI 3, which would take
# the build with it. So `npm audit fix --force` is not a fix here, it is an
# outage, and a gate that fails on the dev-tree count would be red on every run
# forever and get switched off within a month.
#
# What genuinely matters is whether a vulnerability is in something we SHIP.
# That is `npm audit --omit=dev`, and it is 0 — so this blocks on it at
# --audit-level=low, where any regression is real and worth stopping for. The
# dev-tree total is printed every run as information, never as a failure.
#
# The Go side blocks on govulncheck's own exit code, which is non-zero only when
# our code actually CALLS a vulnerable symbol, plus an equality check on the
# module-level findings against the allowlist below (currently one entry). J12's
# "expect exactly one finding, anything beyond that is new" is that allowlist.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Module-level Go findings we have read and accepted. One line per OSV id;
# everything after the id is the reason, and the reason is the point — an entry
# with no argument for being here is a finding nobody triaged.
#
#   GO-2026-5932  golang.org/x/crypto/openpgp — "unmaintained by design". No
#                 fixed version exists, and nothing in our tree imports it; it
#                 arrives under a transitive dependency. Not callable, so
#                 govulncheck reports it at module level only.
GO_ALLOWLIST="GO-2026-5932"

WANT_NPM=1
WANT_GO=1
case "${1:-}" in
  --npm) WANT_GO="" ;;
  --go) WANT_NPM="" ;;
  "") ;;
  *)
    echo "usage: $0 [--npm|--go]" >&2
    exit 2
    ;;
esac

problems=0
say() { printf '%s\n' "$*"; }
fail() {
  problems=$((problems + 1))
  say ""
  say "  ✗ $1"
  [ -n "${2:-}" ] && say "    $2"
}

# ---------------------------------------------------------------- npm --------
if [ -n "$WANT_NPM" ]; then
  say ""
  say "frontend — npm audit"
  say "───────────────────────────────────────────────────────────────────────"

  if [ ! -d frontend/node_modules ]; then
    say "  (no node_modules — auditing from package-lock.json alone)"
  fi

  # BLOCKING half. --omit=dev drops the whole Vue CLI toolchain and leaves the
  # packages that end up in dist. Anything at all here is a finding in shipped
  # code.
  prod_out="$(cd frontend && npm audit --omit=dev --audit-level=low 2>&1)"
  prod_rc=$?
  if [ $prod_rc -ne 0 ]; then
    fail "a SHIPPED frontend dependency has a known vulnerability" \
      "this is the one that reaches users — triage it, don't allowlist it"
    say ""
    printf '%s\n' "$prod_out" | sed 's/^/    /'
  else
    say "  ✓ shipped dependencies (--omit=dev): $(printf '%s' "$prod_out" | tail -1)"
  fi

  # INFORMATIONAL half. Printed so a jump is visible in the run log; never fails
  # the build, for the Vue CLI reason in the header comment.
  dev_total="$(cd frontend && npm audit 2>/dev/null | grep -E '^[0-9]+ vulnerabilities|^found ' | tail -1)"
  say "  · dev toolchain (informational, Vue CLI 5): ${dev_total:-none reported}"

  # vuedraggable publishes its Vue 3 line under the `next` dist-tag and leaves
  # `latest` on the Vue 2 line (latest: 2.24.3, next: 4.1.0 as of 2026-08-11).
  # We run 4.x, which is correct — but it means `npm outdated` reports our
  # version as being AHEAD of latest, and `npm i vuedraggable@latest` is a
  # silent DOWNGRADE to the Vue 2 build that takes drag-and-drop with it (L15).
  #
  # package.json is strict JSON and cannot carry the warning, and a note in a
  # markdown file does not run. This does. Majors, not exact versions: 4.x → 5.x
  # would be a deliberate upgrade, 4.x → 2.x is the accident.
  vd="$(cd frontend && node -p "require('./package-lock.json').packages['node_modules/vuedraggable'].version" 2>/dev/null)"
  vd_major="${vd%%.*}"
  if [ -n "$vd_major" ] && [ "$vd_major" -lt 4 ] 2>/dev/null; then
    fail "vuedraggable is on $vd — that is the VUE 2 line" \
      "npm i vuedraggable@next (4.x). \`@latest\` is 2.x and is a downgrade here."
  else
    say "  ✓ vuedraggable on the Vue 3 line (${vd:-unknown}; \`latest\` is 2.x, ours ships as \`next\`)"
  fi
fi

# ----------------------------------------------------------------- go --------
if [ -n "$WANT_GO" ]; then
  say ""
  say "server — govulncheck"
  say "───────────────────────────────────────────────────────────────────────"

  GOVULN="$(command -v govulncheck || true)"
  [ -z "$GOVULN" ] && [ -x "$(go env GOPATH)/bin/govulncheck" ] && GOVULN="$(go env GOPATH)/bin/govulncheck"
  if [ -z "$GOVULN" ]; then
    fail "govulncheck is not installed" \
      "go install golang.org/x/vuln/cmd/govulncheck@latest"
  else
    # The `grep -v '/scripts'` is not optional: a bare ./... aborts in package
    # loading on the one-off migrations under server/scripts, which reference
    # model shapes from years ago and are deliberately not kept compiling. The
    # abort looks like a broken checkout rather than a filter problem (TODO J12).
    pkgs="$(cd server && go list ./... 2>/dev/null | grep -v '/scripts')"

    # Distinguish "the scan found something" from "the scan never ran". Without
    # this, a package-load abort — which is exactly how J12's bare ./... failed —
    # comes back as a non-zero exit and gets reported as a called vulnerability,
    # sending the reader after a CVE that does not exist.
    if [ -z "$pkgs" ]; then
      fail "go list produced no packages — the scan did not run" \
        "run from a checkout with a working server/ tree; this is not a vulnerability"
    else
      vuln_out="$(cd server && "$GOVULN" $pkgs 2>&1)"
      vuln_rc=$?
      if [ $vuln_rc -ne 0 ]; then
        fail "govulncheck exited $vuln_rc — our code CALLS a vulnerable symbol" \
          "this is reachable in production; fix or upgrade, don't allowlist"
        printf '%s\n' "$vuln_out" | sed 's/^/    /'
      else
        say "  ✓ no called vulnerabilities"
      fi
    fi

    # Module-level findings: govulncheck exits 0 for these, so they need their
    # own comparison or a new one arrives silently. Guarded on $pkgs for the
    # same reason as above — with no packages this compares an empty scan
    # against the allowlist and reports the absence of findings as a change.
    if [ -n "$pkgs" ]; then
      found="$(cd server && "$GOVULN" -format json $pkgs 2>/dev/null |
        python3 -c '
import json,sys
buf=sys.stdin.read(); dec=json.JSONDecoder(); i=0; ids=set()
while i < len(buf):
    while i < len(buf) and buf[i] in " \n\t\r": i += 1
    if i >= len(buf): break
    o, i = dec.raw_decode(buf, i)
    if "finding" in o: ids.add(o["finding"]["osv"])
print(" ".join(sorted(ids)))
')"
      want="$(printf '%s\n' $GO_ALLOWLIST | sort | tr '\n' ' ' | sed 's/ $//')"
      have="$(printf '%s\n' $found | sort | tr '\n' ' ' | sed 's/ $//')"
      if [ "$have" = "$want" ]; then
        say "  ✓ module-level findings match the allowlist: ${want:-none}"
      else
        fail "module-level findings changed" \
          "expected: ${want:-none}
         found:    ${have:-none}
         A new id means a new advisory — read it, then either fix it or add it
         to GO_ALLOWLIST in this script WITH the reason it is acceptable."
      fi
    fi
  fi
fi

say ""
if [ $problems -eq 0 ]; then
  say "dependency-audit: PASS"
  exit 0
fi
say "dependency-audit: $problems problem(s)"
exit 1
