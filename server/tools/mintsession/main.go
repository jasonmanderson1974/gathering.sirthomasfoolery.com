// Command mintsession prints a session cookie value that this server will
// accept, so a browser can be driven as a signed-in user without SMTP (OTP) or
// Google OAuth — neither of which is wired up in local development.
//
// It grants no capability that SESSION_SECRET does not already grant: anyone
// holding that secret can forge a session by definition. Guard the secret, not
// this tool. It is a development convenience and is never invoked by the server.
//
// It deliberately lives inside the server module rather than standing alone, so
// it resolves the SAME gorilla/sessions version the server does. A separate
// module could drift to a different encoding and mint cookies that look right
// and are silently rejected.
//
//	Usage: SESSION_SECRET=... go run ./tools/mintsession <userIdHex>
//
// The user must also exist in Mongo and be on the allowlist — AuthRequired
// enforces the roll on every request, not just at sign-in.
package main

import (
	"fmt"
	"os"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

func main() {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "SESSION_SECRET is required (match the running server's)")
		os.Exit(2)
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: SESSION_SECRET=... go run ./tools/mintsession <userIdHex>")
		os.Exit(2)
	}
	userId := os.Args[1]

	// Mirrors main.go: cookie.NewStore(secret) -> sessions.NewCookieStore(secret).
	// One key pair means hash-only (no encryption), which is what the server does.
	store := sessions.NewCookieStore([]byte(secret))

	// gorilla/sessions stores Values as map[interface{}]interface{}, and the
	// session cookie is named "session" (see main.go's sessions.Sessions call).
	values := map[interface{}]interface{}{"userId": userId}

	encoded, err := securecookie.EncodeMulti("session", values, store.Codecs...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
	fmt.Print(encoded)
}
