package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessaging(t *testing.T) {
	t.Run("OrList", func(t *testing.T) {
		assert.Equal(t, "", OrList())
		assert.Equal(t, "A", OrList("A"))
		assert.Equal(t, "A or B", OrList("A", "B"))
		assert.Equal(t, "A, B, or C", OrList("A", "B", "C"))
		assert.Equal(t, "A, B, C, or D", OrList("A", "B", "C", "D"))
		assert.Equal(t, "A, B, C, D, or E", OrList("A", "B", "C", "D", "E", "F"))
	})

	t.Run("QuotedOrList", func(t *testing.T) {
		assert.Equal(t, ``, QuotedOrList())
		assert.Equal(t, `"A"`, QuotedOrList("A"))
		assert.Equal(t, `"A" or "B"`, QuotedOrList("A", "B"))
		assert.Equal(t, `"A", "B", or "C"`, QuotedOrList("A", "B", "C"))
		assert.Equal(t, `"A", "B", "C", or "D"`, QuotedOrList("A", "B", "C", "D"))
	})
}
