package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTags_ToScaling(t *testing.T) {
	t.Run("given asg-schedule tag with when value is correct then scaling is returned without error", func(t *testing.T) {
		tags := Tags{tagKey: "desired:2,max:5,min:1", "another-tag": "test"}
		scaling, err := tags.ToScaling()
		require.NoError(t, err)

		assert.Equal(t, int32(1), scaling.MinSize)
		assert.Equal(t, int32(5), scaling.MaxSize)
		assert.Equal(t, int32(2), scaling.DesiredCapacity)
	})

	t.Run("given asg-schedule tag when value is different order then scaling is returned without error", func(t *testing.T) {
		tags := Tags{tagKey: "max:5,desired:2,min:1"}
		scaling, err := tags.ToScaling()
		require.NoError(t, err)

		assert.Equal(t, int32(1), scaling.MinSize)
		assert.Equal(t, int32(5), scaling.MaxSize)
		assert.Equal(t, int32(2), scaling.DesiredCapacity)
	})

	t.Run("given asg-schedule tag when value has spaces then scaling is returned without error", func(t *testing.T) {
		tags := Tags{tagKey: "desired :2 ,max:5, min : 1"}
		scaling, err := tags.ToScaling()
		require.NoError(t, err)

		assert.Equal(t, int32(1), scaling.MinSize)
		assert.Equal(t, int32(5), scaling.MaxSize)
		assert.Equal(t, int32(2), scaling.DesiredCapacity)
	})

	t.Run("given asg-schedule tag when value is incorrect then error is returned", func(t *testing.T) {
		tags := Tags{tagKey: "des:2,max:5,min:1"}
		_, err := tags.ToScaling()
		require.Error(t, err)
	})

	t.Run("given asg-schedule when tag is missing then error is returned", func(t *testing.T) {
		tags := Tags{"another-tag": "test"}
		_, err := tags.ToScaling()
		require.Error(t, err)
	})
}
