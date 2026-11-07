package list

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// noGapItem is a fixed-height item that opts out of the inter-item gap.
type noGapItem struct {
	*Versioned
	id     string
	height int
}

func newNoGapItem(id string, height int) *noGapItem {
	return &noGapItem{Versioned: NewVersioned(), id: id, height: height}
}

func (n *noGapItem) Render(_ int) string {
	parts := make([]string, n.height)
	for i := range n.height {
		parts[i] = n.id + ":" + fmt.Sprint(i)
	}
	return strings.Join(parts, "\n")
}

func (n *noGapItem) Finished() bool { return true }

func (n *noGapItem) NoGap() bool { return true }

// TestList_NoGapBetweenNoGapItems pins that two adjacent NoGap items render
// without the inter-item gap row, while a NoGap item followed by a plain
// item keeps the gap. The trailing gap after the last item is pre-existing
// render behavior and stays.
func TestList_NoGapBetweenNoGapItems(t *testing.T) {
	t.Parallel()

	gap := newNoGapItem("a", 2)
	gap2 := newNoGapItem("b", 2)
	l := NewList(gap, gap2)
	l.SetSize(40, 20)
	l.SetGap(1)

	out := l.Render()
	require.Equal(t, strings.Join([]string{
		"a:0",
		"a:1",
		"b:0",
		"b:1",
		"",
	}, "\n"), out)
}

func TestList_NoGapKeptTowardPlainItems(t *testing.T) {
	t.Parallel()

	plain := newMultiLineItem("a", 2)
	gap := newNoGapItem("b", 2)
	gap2 := newNoGapItem("c", 2)
	l := NewList(plain, gap, gap2)
	l.SetSize(40, 20)
	l.SetGap(1)

	out := l.Render()
	require.Equal(t, strings.Join([]string{
		"a:0",
		"a:1",
		"",
		"b:0",
		"b:1",
		"c:0",
		"c:1",
		"",
	}, "\n"), out)
}

// TestList_NoGapScrollConsistency pins that Offset and TotalHeight agree
// with the rendered layout when suppressed pairs exist: scrolling to an
// offset lands exactly at an item boundary.
func TestList_NoGapScrollConsistency(t *testing.T) {
	t.Parallel()

	items := []Item{
		newNoGapItem("a", 2),
		newNoGapItem("b", 2),
		newMultiLineItem("c", 2),
	}
	l := NewList(items...)
	l.SetSize(40, 20)
	l.SetGap(1)

	require.Equal(t, 7, l.TotalHeight())

	l.offsetIdx = 2
	l.offsetLine = 0
	require.Equal(t, 5, l.Offset())
}
