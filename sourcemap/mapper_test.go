package sourcemap

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
)

func TestNewSmapMapper(t *testing.T) {
	mapper, err := NewSmapMapper(Config{})
	assert.Nil(t, mapper)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, InitError)

	mapper, err = NewSmapMapper(getFakeConfig())
	assert.NoError(t, err)
	assert.NotNil(t, mapper)
}

func TestApply(t *testing.T) {
	mapper, err := NewSmapMapper(getFakeConfig())
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// error occurs
	mapping, err := mapper.Apply(Id{}, 0, 0)
	assert.Nil(t, mapping)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, KeyError)

	// no mapping found in sourcemap
	mapper.Accessor = &fakeAccessor{}
	mapping, err = mapper.Apply(Id{Path: "bundle.js.map"}, 0, 0)
	assert.Nil(t, mapping)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No Sourcemap found")
	assert.Equal(t, (err.(Error)).Kind, KeyError)

	// mapping found in minified sourcemap
	mapping, err = mapper.Apply(Id{Path: "bundle.js.map"}, 1, 7)
	assert.NoError(t, err)
	assert.NotNil(t, mapping)
	assert.Equal(t, "webpack:///bundle.js", mapping.Filename)
	assert.Equal(t, "", mapping.Function)
	assert.Equal(t, 1, mapping.Lineno)
	assert.Equal(t, 9, mapping.Colno)
	assert.Equal(t, "bundle.js.map", mapping.Path)
}

type fakeAccessor struct{}

func (ac *fakeAccessor) Fetch(smapId Id) (*sourcemap.Consumer, error) {
	current, _ := os.Getwd()
	path := filepath.Join(current, "../tests/data/valid/sourcemap/", smapId.Path)
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return sourcemap.Parse("", fileBytes)
}
func (ac *fakeAccessor) Remove(smapId Id) {}
