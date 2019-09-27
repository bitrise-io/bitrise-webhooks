package middleware

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitrise-io/api-utils/httpresponse"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/require"
)

// TestCase ...
type TestCase struct {
	RequestHeaders   map[string]string
	RequestBody      interface{}
	ExpectedStatus   int
	ExpectedResponse interface{}
	Middleware       alice.Chain
}

// PerformTest ...
func PerformTest(t *testing.T,
	httpMethod, url string,
	tc TestCase,
) {
	t.Helper()

	ts := httptest.NewServer(tc.Middleware.Then(TestHandler()))
	defer ts.Close()

	var u bytes.Buffer
	u.WriteString(string(ts.URL))
	u.WriteString(url)

	var bodyBytes []byte
	bodyBytes, err := json.Marshal(tc.RequestBody)
	require.NoError(t, err)

	req, err := http.NewRequest(httpMethod, u.String(), bytes.NewBuffer(bodyBytes))
	require.NoError(t, err)

	for key, val := range tc.RequestHeaders {
		req.Header.Add(key, val)
	}
	client := http.Client{}

	res, err := client.Do(req)
	require.NoError(t, err)

	b, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)

	require.Equal(t, tc.ExpectedStatus, res.StatusCode)
	if tc.ExpectedResponse != nil {
		expectedBytes, err := json.Marshal(tc.ExpectedResponse)
		require.NoError(t, err)
		require.Equal(t, string(expectedBytes), strings.Trim(string(b), "\n"))
	}
}

// TestHandler ...
func TestHandler() http.HandlerFunc {
	fn := func(rw http.ResponseWriter, req *http.Request) {
		httpresponse.RespondWithSuccessNoErr(rw, map[string]string{"message": "Success"})
	}
	return http.HandlerFunc(fn)
}
