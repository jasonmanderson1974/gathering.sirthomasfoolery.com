package auth

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	IdToken          string `json:"id_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	// Error and ErrorDescription mirror TokenResponse above: both Google and
	// Microsoft report a failed refresh as a *string* code ("invalid_grant",
	// "invalid_client") plus a human-readable description. Typing Error as an
	// object made the whole decode fail, so the caller was handed a JSON type
	// error and the real reason — revoked consent, expired refresh token, bad
	// client credentials — was thrown away.
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
