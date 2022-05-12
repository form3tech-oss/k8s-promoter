package git_test

import (
	"testing"

	"github.com/form3tech/k8s-promoter/internal/git"
	"github.com/stretchr/testify/require"
)

func TestNewCommitRange(t *testing.T) {
	tests := map[string]struct {
		commitRange string
		cr          *git.CommitRange
		err         error
	}{
		"when commit range only one sha reference": {
			commitRange: "992483b570a9cbf2dfa279865935981b6f468a82",
			err:         git.ErrCommitRangeIncorrect,
		},
		"when commit range has incorrect separator": {
			commitRange: "992483b570a9..08759b4b4d6c",
			err:         git.ErrCommitRangeIncorrect,
		},
		"when commit range is of prefix hashes separator": {
			commitRange: "992483b570a9...08759b4b4d6c",
			cr: &git.CommitRange{
				FromPrefix: "992483b570a9",
				ToPrefix:   "08759b4b4d6c",
			},

			err: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cr, err := git.NewCommitRange(tt.commitRange)

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				require.Nil(t, cr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.cr, cr)
		})
	}
}
