package hook

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_requestLogFields(t *testing.T) {
	t.Log("GitHub delivery")
	{
		request, err := http.NewRequest("POST", "https://hooks.bitrise.io/h/github/app-slug/api-token", nil)
		require.NoError(t, err)
		request.Header.Set("X-GitHub-Delivery", "eed925c4-9fba-11f1-80ee-fadabd1c2fce")

		fields := requestLogFields(request, "github", "app-slug")

		require.Equal(t, []zap.Field{
			zap.String("service_id", "github"),
			zap.String("app_slug", "app-slug"),
			zap.String("github_delivery_id", "eed925c4-9fba-11f1-80ee-fadabd1c2fce"),
		}, fields)
	}

	t.Log("Request without a delivery ID")
	{
		request, err := http.NewRequest("POST", "https://hooks.bitrise.io/h/gitlab/app-slug/api-token", nil)
		require.NoError(t, err)

		fields := requestLogFields(request, "gitlab", "app-slug")

		require.Equal(t, []zap.Field{
			zap.String("service_id", "gitlab"),
			zap.String("app_slug", "app-slug"),
		}, fields)
	}
}
