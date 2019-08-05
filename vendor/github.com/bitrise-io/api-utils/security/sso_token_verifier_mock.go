package security

// SsoTokenVerifierMock ...
type SsoTokenVerifierMock struct {
	VerifyFn func(timestamp, ssoToken, appSlug string) (bool, error)
}

// Verify ...
func (v *SsoTokenVerifierMock) Verify(timestamp, ssoToken, appSlug string) (bool, error) {
	if v.VerifyFn == nil {
		panic("You have to override SsoTokenVerifier.Verify function in tests")
	}
	return v.VerifyFn(timestamp, ssoToken, appSlug)
}
