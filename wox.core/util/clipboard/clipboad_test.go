package clipboard

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_Copy(t *testing.T) {
	randomStr := uuid.NewString()
	err := WriteText(randomStr)

	assert.NoError(t, err)

	data, readErr := Read()
	assert.NoError(t, readErr)
	assert.Equal(t, randomStr, data.String())
}
