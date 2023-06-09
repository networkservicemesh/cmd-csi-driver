package version_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/networkservicemesh/cmd-csi-driver/internal/version"
)

func Test(t *testing.T) {
	versionData, err := os.ReadFile("VERSION")
	require.NoError(t, err)

	actual := version.Version()
	expectedPrefix := strings.TrimSpace(string(versionData))

	require.True(t, strings.HasPrefix(actual, expectedPrefix), "version %q should have prefix %q", actual, expectedPrefix)
}
