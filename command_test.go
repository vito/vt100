package vt100_test

import (
	"io"
	"strings"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	. "github.com/vito/vt100"
	"github.com/vito/vt100/vttest"
)

func splitLines(s string) [][]rune {
	ss := strings.Split(s, "\n")
	r := make([][]rune, len(ss))
	for i, line := range ss {
		r[i] = []rune(line)
	}
	return r
}

func esc(s string) string {
	return "\u001b" + s
}

func cmd(s string) Command {
	cmd, err := Decode(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	return cmd
}

func cmds(s string) []Command {
	var c []Command
	r := strings.NewReader(s)
	for {
		x, err := Decode(r)
		if err == io.EOF {
			return c
		}
		if err != nil {
			panic(err)
		}
		c = append(c, x)
	}
}

func TestPutRune(t *testing.T) {
	v := vttest.FromLines("abc\ndef\nghi")
	v.Cursor.Y = 1
	v.Cursor.X = 1

	assert.Nil(t, v.Process(cmd("z")))
	assert.Equal(t, splitLines("abc\ndzf\nghi"), v.Content)
	assert.Equal(t, 2, v.Cursor.X)
	assert.Equal(t, 1, v.Cursor.Y)
}

func TestMoveCursor(t *testing.T) {
	v := vttest.FromLines("abc\ndef\nghi")
	assert.Nil(t, v.Process(cmd(esc("[3;1H"))))
	assert.Equal(t, 2, v.Cursor.Y)
	assert.Equal(t, 0, v.Cursor.X)
}

func TestCursorDirections(t *testing.T) {
	v := vttest.FromLines("abc\ndef\nghi")

	moves := strings.Join([]string{
		esc("[2B"), // down down
		esc("[2C"), // right right
		esc("[A"),  // up (no args = 1)
		esc("[1D"), // left
	}, "") // End state: +1, +1
	s := strings.NewReader(moves)

	want := []Cursor{
		{Y: 2, X: 0},
		{Y: 2, X: 2},
		{Y: 1, X: 2},
		{Y: 1, X: 1},
	}
	var got []Cursor

	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		got = append(got, v.Cursor)
		cmd, err = Decode(s)
	}
	if assert.Equal(t, err, io.EOF) {
		assert.Equal(t, want, got)
	}
}

func TestErase(t *testing.T) {
	c := Format{Fg: termenv.ANSIYellow, Intensity: Bold}
	var d Format
	for _, tc := range []struct {
		command Command
		want    *VT100
	}{
		{cmd(esc("[K")), vttest.FromLinesAndFormats("abcd\nef  \nijkl", [][]Format{
			{c, c, c, c},
			{c, c, d, d},
			{c, c, c, c},
		})},
		{cmd(esc("[1K")), vttest.FromLinesAndFormats("abcd\n   h\nijkl", [][]Format{
			{c, c, c, c},
			{d, d, d, c},
			{c, c, c, c},
		})},
		{cmd(esc("[2K")), vttest.FromLinesAndFormats("abcd\n    \nijkl", [][]Format{
			{c, c, c, c},
			{d, d, d, d},
			{c, c, c, c},
		})},
		{cmd(esc("[J")), vttest.FromLinesAndFormats("abcd\n    \n    ", [][]Format{
			{c, c, c, c},
			{d, d, d, d},
			{d, d, d, d},
		})},
		{cmd(esc("[1J")), vttest.FromLinesAndFormats("    \n    \nijkl", [][]Format{
			{d, d, d, d},
			{d, d, d, d},
			{c, c, c, c},
		})},
		{cmd(esc("[2J")), vttest.FromLinesAndFormats("    \n    \n    ", [][]Format{
			{d, d, d, d},
			{d, d, d, d},
			{d, d, d, d},
		})},
	} {
		v := vttest.FromLinesAndFormats(
			"abcd\nefgh\nijkl", [][]Format{
				{c, c, c, c},
				{c, c, c, c},
				{c, c, c, c},
			})
		v.Cursor = Cursor{Y: 1, X: 2}
		beforeCursor := v.Cursor

		assert.Nil(t, v.Process(tc.command))
		assert.Equal(t, tc.want.Content, v.Content, "while evaluating ", tc.command)
		assert.Equal(t, tc.want.Format, v.Format, "while evaluating ", tc.command)
		// Check the cursor separately. We don't set it on any of the test cases
		// so we cannot expect it to be equal. It's not meant to change.
		assert.Equal(t, beforeCursor, v.Cursor)
	}
}

var (
	bs = "\u0008" // Use strings to contain these runes so they can be concatenated easily.
	lf = "\u000a"
	cr = "\u000d"

	tab = "\t"
)

func TestBackspace(t *testing.T) {
	v := vttest.FromLines("BA..")
	v.Cursor.Y, v.Cursor.X = 0, 2

	backspace := cmd(bs)
	assert.Nil(t, v.Process(backspace))
	// Backspace doesn't actually delete text.
	assert.Equal(t, vttest.FromLines("BA..").Content, v.Content)
	assert.Equal(t, 1, v.Cursor.X)

	v.Cursor.X = 0
	assert.Nil(t, v.Process(backspace))
	assert.Equal(t, 0, v.Cursor.X)

	v = vttest.FromLines("..\n..")
	v.Cursor.Y, v.Cursor.X = 1, 0
	assert.Nil(t, v.Process(backspace))
	assert.Equal(t, 0, v.Cursor.Y)
	assert.Equal(t, 1, v.Cursor.X)
}

func TestLineFeed(t *testing.T) {
	v := vttest.FromLines("AA\n..")
	v.Cursor.X = 1

	for _, c := range cmds(lf + "b") {
		assert.Nil(t, v.Process(c))
	}
	assert.Equal(t, vttest.FromLines("AA\nb.").Content, v.Content)
}

func TestHorizontalTab(t *testing.T) {
	v := vttest.FromLines("AA          \n")
	v.Cursor.X = 2

	for _, c := range cmds(tab + "b" + tab + "c" + tab + "d" + tab + "e" + tab + "f") {
		assert.Nil(t, v.Process(c))
	}

	assert.Equal(t, vttest.FromLines("AA  b   c  d\n    e   f").Content, v.Content)

	v.Cursor.X = 0
	v.Cursor.Y = 1
	for _, c := range cmds(tab + "x" + tab + "y") {
		assert.Nil(t, v.Process(c))
	}

	assert.Equal(t, vttest.FromLines("AA  b   c  d\n    x   y").Content, v.Content)
}

func TestCarriageReturn(t *testing.T) {
	v := vttest.FromLines("AA\n..")
	v.Cursor.X = 1

	for _, c := range cmds(cr + "b") {
		assert.Nil(t, v.Process(c))
	}
	assert.Equal(t, vttest.FromLines("bA\n..").Content, v.Content)
}

func TestAttributes(t *testing.T) {
	v := vttest.FromLines("....")
	s := strings.NewReader(
		esc("[2ma") + esc("[5;22;31mb") + esc("[0mc") + esc("[4;46md"))
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []rune("abcd"), v.Content[0])
	assert.Equal(t, []Format{
		{Intensity: Faint}, {Blink: true, Fg: termenv.ANSIRed}, {Reset: true}, {Reset: true, Underline: true, Bg: termenv.ANSICyan},
	}, v.Format[0])
}

func TestEmptyReset(t *testing.T) {
	v := vttest.FromLines("....")
	s := strings.NewReader(
		esc("[2ma") + esc("[5;22;31mb") + esc("[mc") + esc("[4;46md"))
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []rune("abcd"), v.Content[0])
	assert.Equal(t, []Format{
		{Intensity: Faint}, {Blink: true, Fg: termenv.ANSIRed}, {Reset: true}, {Reset: true, Underline: true, Bg: termenv.ANSICyan},
	}, v.Format[0])
}

func TestBold(t *testing.T) {
	v := vttest.FromLines("...")
	s := strings.NewReader(esc("[1ma") + esc("[31mb") + esc("[91mc"))
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []rune("abc"), v.Content[0])
	assert.Equal(t, []Format{
		{Intensity: Bold}, {Fg: termenv.ANSIRed, Intensity: Bold}, {Fg: termenv.ANSIBrightRed, Intensity: Bold},
	}, v.Format[0])
}

func TestBrightFg(t *testing.T) {
	v := vttest.FromLines("...\n...")
	s := strings.NewReader(
		esc("[90ma") + esc("[91mb") + esc("[97mc"),
	)
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []rune("abc"), v.Content[0])
	assert.Equal(t, []Format{
		{Fg: termenv.ANSIBrightBlack}, {Fg: termenv.ANSIBrightRed}, {Fg: termenv.ANSIBrightWhite},
	}, v.Format[0])
}

func TestBrightBg(t *testing.T) {
	v := vttest.FromLines("...\n...")
	s := strings.NewReader(
		esc("[100ma") + esc("[101mb") + esc("[107mc"),
	)
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, []rune("abc"), v.Content[0])
	assert.Equal(t, []Format{
		{Bg: termenv.ANSIBrightBlack}, {Bg: termenv.ANSIBrightRed}, {Bg: termenv.ANSIBrightWhite},
	}, v.Format[0])
}

func TestAutoResizeX(t *testing.T) {
	v := NewVT100(1, 1)
	v.AutoResizeX = true
	s := strings.NewReader("abcde")
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, "abcde", string(v.Content[0]))
	assert.Equal(t, len("abcde"), v.Width)
	assert.Equal(t, 1, v.Height)
	assert.Equal(t, []Format{
		{},
		{},
		{},
		{},
		{},
	}, v.Format[0])
}

func TestAutoResizeY(t *testing.T) {
	v := NewVT100(1, 1)
	v.AutoResizeY = true
	s := strings.NewReader("abcde")
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 1, v.Width)
	assert.Equal(t, 5, v.Height)
	assert.Equal(t, "a", string(v.Content[0]))
	assert.Equal(t, []Format{{}}, v.Format[0])
	assert.Equal(t, "b", string(v.Content[1]))
	assert.Equal(t, []Format{{}}, v.Format[1])
	assert.Equal(t, "c", string(v.Content[2]))
	assert.Equal(t, []Format{{}}, v.Format[2])
	assert.Equal(t, "d", string(v.Content[3]))
	assert.Equal(t, []Format{{}}, v.Format[3])
	assert.Equal(t, "e", string(v.Content[4]))
	assert.Equal(t, []Format{{}}, v.Format[4])
}

func TestAutoResizeXY(t *testing.T) {
	v := NewVT100(1, 1)
	v.AutoResizeX = true
	v.AutoResizeY = true
	s := strings.NewReader("abcde\n12345")
	cmd, err := Decode(s)
	for err == nil {
		assert.Nil(t, v.Process(cmd))
		cmd, err = Decode(s)
	}
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 5, v.Width)
	assert.Equal(t, 2, v.Height)
	assert.Equal(t, "abcde", string(v.Content[0]))
	assert.Equal(t, []Format{{}, {}, {}, {}, {}}, v.Format[0])
	assert.Equal(t, "12345", string(v.Content[1]))
	assert.Equal(t, []Format{{}, {}, {}, {}, {}}, v.Format[1])
}
