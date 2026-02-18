package github

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelMutations_RejectManagedBotLabels(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	err := client.AddLabel(ctx, 1, "bot:fix")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrManagedBotLabel))

	err = client.RemoveLabel(ctx, 1, "bot:fix")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrManagedBotLabel))

	err = client.ReplaceLabel(ctx, 1, "bot:fix", "bug")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrManagedBotLabel))

	_, err = client.ReplaceLabelIfPresent(ctx, 1, "bot:fix", "bug")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrManagedBotLabel))
}
