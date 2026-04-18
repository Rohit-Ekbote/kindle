package presets_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/emdash/kindle/internal/presets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPresetsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestLoader_Load(t *testing.T) {
	loader := presets.NewLoader(testPresetsDir())
	list, err := loader.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "small-dev", list[0].Name)
	assert.Equal(t, "e2-standard-2", list[0].Defaults.MachineType)
	assert.Equal(t, int64(50), list[0].Defaults.DiskSizeGB)
	assert.Len(t, list[0].UserEditable, 3)
}

func TestLoader_Get_NotFound(t *testing.T) {
	loader := presets.NewLoader(testPresetsDir())
	_, err := loader.Get("nonexistent")
	assert.ErrorIs(t, err, presets.ErrPresetNotFound)
}

func TestLoader_BaseValues(t *testing.T) {
	loader := presets.NewLoader(testPresetsDir())
	preset, err := loader.Get("small-dev")
	require.NoError(t, err)
	vals, err := preset.BaseValues()
	require.NoError(t, err)
	assert.NotEmpty(t, vals)
}
