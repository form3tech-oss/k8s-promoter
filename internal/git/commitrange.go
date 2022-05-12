package git

import (
	"errors"
	"fmt"
	"strings"
)

const (
	sep = "..."
)

var ErrCommitRangeIncorrect = errors.New("commit range incorrect")

type CommitRange struct {
	FromPrefix string
	ToPrefix   string
}

func NewCommitRange(r string) (*CommitRange, error) {
	s := strings.Split(r, sep)
	if len(s) != 2 {
		return nil, fmt.Errorf("%w: incorrect range found: %v", ErrCommitRangeIncorrect, s)
	}

	return &CommitRange{
		FromPrefix: s[0],
		ToPrefix:   s[1],
	}, nil
}

func (c *CommitRange) TargetRefPrefix() string {
	return c.ToPrefix
}
