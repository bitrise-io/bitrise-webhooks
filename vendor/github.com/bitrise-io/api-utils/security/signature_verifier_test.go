package security_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bitrise-io/api-utils/security"
)

func Test_SignatureVerifier_Verify(t *testing.T) {
	t.Log("ok - true")
	{
		testSecret := "89fa2fae2f7c89eca9d4dc1406ee5e1a"
		testPayloadBody := `{"message":"test content"}`
		testPayloadSignature := `sha1=c48856de832335695d0d1d4e3bca31d4687c0e0d`

		signatureVerfier := security.NewSignatureVerifier(testSecret, testPayloadBody, testPayloadSignature)

		require.True(t, signatureVerfier.Verify())
	}
	t.Log("ok - false")
	{
		testSecret := "89fa2fae2f7c89eca9d4dc1406ee5e1a"
		testPayloadBody := `{"message":"test content"}`
		testPayloadSignature := `sha1=invalid-signature`

		signatureVerfier := security.NewSignatureVerifier(testSecret, testPayloadBody, testPayloadSignature)

		require.False(t, signatureVerfier.Verify())
	}
}
