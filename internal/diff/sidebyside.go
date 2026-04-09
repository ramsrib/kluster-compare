package diff

import (
	"strings"
)

// DiffLine represents one row in a side-by-side diff.
type DiffLine struct {
	Left    string // empty if only added on right
	Right   string // empty if only removed on left
	LeftNo  int    // 0 means no line number
	RightNo int
	Kind    LineKind
}

type LineKind int

const (
	LineEqual   LineKind = iota
	LineChanged          // left removed, right added (paired)
	LineRemoved          // only on left
	LineAdded            // only on right
	LineHunk             // @@ hunk header
)

// SideBySide parses a unified diff string into paired left/right lines
// suitable for side-by-side rendering.
func SideBySide(unifiedDiff string) []DiffLine {
	lines := strings.Split(unifiedDiff, "\n")
	var result []DiffLine

	var removeBuf []string
	var addBuf []string
	leftNo, rightNo := 0, 0

	flushBuffers := func() {
		// Pair up removes and adds
		maxLen := len(removeBuf)
		if len(addBuf) > maxLen {
			maxLen = len(addBuf)
		}
		for i := 0; i < maxLen; i++ {
			dl := DiffLine{}
			if i < len(removeBuf) && i < len(addBuf) {
				dl.Kind = LineChanged
				dl.Left = removeBuf[i]
				dl.Right = addBuf[i]
				leftNo++
				rightNo++
				dl.LeftNo = leftNo
				dl.RightNo = rightNo
			} else if i < len(removeBuf) {
				dl.Kind = LineRemoved
				dl.Left = removeBuf[i]
				leftNo++
				dl.LeftNo = leftNo
			} else {
				dl.Kind = LineAdded
				dl.Right = addBuf[i]
				rightNo++
				dl.RightNo = rightNo
			}
			result = append(result, dl)
		}
		removeBuf = removeBuf[:0]
		addBuf = addBuf[:0]
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			// skip file headers
			continue

		case strings.HasPrefix(line, "@@"):
			flushBuffers()
			// Parse line numbers from @@ -a,b +c,d @@
			leftNo, rightNo = parseHunkHeader(line)
			leftNo-- // will be incremented on first context line
			rightNo--
			result = append(result, DiffLine{
				Kind: LineHunk,
				Left: line,
			})

		case strings.HasPrefix(line, "-"):
			removeBuf = append(removeBuf, line[1:])

		case strings.HasPrefix(line, "+"):
			addBuf = append(addBuf, line[1:])

		default:
			flushBuffers()
			if line == "" && len(result) > 0 {
				// trailing newline, skip
				continue
			}
			// Context line (starts with space or is plain text)
			text := line
			if strings.HasPrefix(text, " ") {
				text = text[1:]
			}
			leftNo++
			rightNo++
			result = append(result, DiffLine{
				Kind:    LineEqual,
				Left:    text,
				Right:   text,
				LeftNo:  leftNo,
				RightNo: rightNo,
			})
		}
	}
	flushBuffers()

	return result
}

func parseHunkHeader(line string) (int, int) {
	// @@ -leftStart,leftCount +rightStart,rightCount @@
	left, right := 1, 1

	// Find left start
	if idx := strings.Index(line, "-"); idx >= 0 {
		rest := line[idx+1:]
		fmt2 := 0
		for _, c := range rest {
			if c >= '0' && c <= '9' {
				fmt2 = fmt2*10 + int(c-'0')
			} else {
				break
			}
		}
		if fmt2 > 0 {
			left = fmt2
		}
	}

	// Find right start
	if idx := strings.Index(line, "+"); idx >= 0 {
		rest := line[idx+1:]
		fmt2 := 0
		for _, c := range rest {
			if c >= '0' && c <= '9' {
				fmt2 = fmt2*10 + int(c-'0')
			} else {
				break
			}
		}
		if fmt2 > 0 {
			right = fmt2
		}
	}

	return left, right
}
