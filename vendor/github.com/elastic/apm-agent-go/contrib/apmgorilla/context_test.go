package apmgorilla

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMassageTemplate(t *testing.T) {
	out := massageTemplate("/articles/{category}/{id:[0-9]+}")
	assert.Equal(t, "/articles/{category}/{id}", out)
}
