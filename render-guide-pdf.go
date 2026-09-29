// render-guide-pdf produces a styled, searchable PDF using Go base-14 fonts.
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
)

const (
	pw, ph            = 612, 792
	left, top, bottom = 48, 88, 54
)

type line struct{ text, kind string }

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run render-guide-pdf.go input.md output.pdf")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2], pdf(layout(parse(string(b)))), 0644); err != nil {
		panic(err)
	}
}
func parse(md string) []line {
	var o []line
	for _, r := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		s := strings.TrimSpace(r)
		k, w := "body", 86
		switch {
		case strings.HasPrefix(s, "# "):
			k, w, s = "title", 42, strings.TrimPrefix(s, "# ")
		case strings.HasPrefix(s, "## "):
			k, w, s = "h1", 58, strings.TrimPrefix(s, "## ")
		case strings.HasPrefix(s, "### "):
			k, w, s = "h2", 68, strings.TrimPrefix(s, "### ")
		case s == "":
			o = append(o, line{"", "space"})
			continue
		case strings.HasPrefix(s, "`"):
			k, w, s = "code", 70, strings.Trim(s, "`")
		case len(s) > 2 && unicode.IsDigit(rune(s[0])) && s[1] == '.':
			k, w = "list", 82
		}
		for _, p := range wrap(ascii(s), w) {
			o = append(o, line{p, k})
		}
	}
	return o
}
func wrap(s string, w int) []string {
	if len(s) <= w {
		return []string{s}
	}
	var o []string
	for len(s) > w {
		c := strings.LastIndex(s[:w+1], " ")
		if c < 1 {
			c = w
		}
		o, s = append(o, s[:c]), strings.TrimLeft(s[c:], " ")
	}
	return append(o, s)
}
func ascii(s string) string {
	return strings.Map(func(r rune) rune {
		if r > 126 {
			return '-'
		}
		return r
	}, s)
}

func layout(ls []line) []string {
	var pages []string
	var b strings.Builder
	page := 1
	y := float64(ph - top)
	begin := func() {
		b.Reset()
		b.WriteString("0.06 0.18 0.32 rg 0 748 612 44 re f\n")
		tx(&b, "Azure onboarding automation", 48, 764, 10, "F2", "1 1 1")
		tx(&b, "Implementation guide", 48, 751, 8, "F1", "0.80 0.89 0.96")
		y = float64(ph - top)
	}
	finish := func() {
		tx(&b, "Internal engineering guide", 48, 30, 8, "F1", "0.40 0.45 0.50")
		tx(&b, fmt.Sprintf("%d", page), 556, 30, 8, "F1", "0.40 0.45 0.50")
		pages = append(pages, b.String())
		page++
	}
	begin()
	for _, l := range ls {
		h := ht(l.kind)
		if y-h < bottom {
			finish()
			begin()
		}
		switch l.kind {
		case "space":
			y -= 6
		case "title":
			tx(&b, l.text, left, y, 24, "F2", "0.06 0.18 0.32")
			y -= h
		case "h1":
			b.WriteString(fmt.Sprintf("0.12 0.48 0.66 rg %d %.1f 44 3 re f\n", left, y-4))
			tx(&b, l.text, left+56, y-1, 15, "F2", "0.06 0.18 0.32")
			y -= h
		case "h2":
			tx(&b, l.text, left, y, 11, "F2", "0.10 0.39 0.56")
			y -= h
		case "code":
			b.WriteString(fmt.Sprintf("0.94 0.96 0.98 rg %d %.1f %d 17 re f\n", left, y-12, pw-left-left))
			tx(&b, l.text, left+9, y-1, 8.5, "F3", "0.12 0.20 0.28")
			y -= h
		case "list":
			tx(&b, l.text, left+8, y, 9.5, "F1", "0.16 0.19 0.22")
			y -= h
		default:
			tx(&b, l.text, left, y, 9.5, "F1", "0.16 0.19 0.22")
			y -= h
		}
	}
	finish()
	return pages
}
func ht(k string) float64 {
	switch k {
	case "title":
		return 34
	case "h1":
		return 27
	case "h2":
		return 21
	case "code":
		return 21
	case "space":
		return 6
	}
	return 14
}
func tx(b *strings.Builder, s string, x int, y, size float64, font, color string) {
	fmt.Fprintf(b, "BT /%s %.1f Tf %s rg 1 0 0 1 %d %.1f Tm (%s) Tj ET\n", font, size, color, x, y, esc(s))
}

func pdf(pages []string) []byte {
	var o bytes.Buffer
	o.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	objs := make([]string, 5+len(pages)*2)
	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 6+i*2)
	}
	objs[0] = "<< /Type /Catalog /Pages 2 0 R /PageLayout /OneColumn >>"
	objs[1] = "<< /Type /Pages /Kids [" + strings.Join(kids, " ") + "] /Count " + fmt.Sprint(len(pages)) + " >>"
	objs[2] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	objs[3] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>"
	objs[4] = "<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>"
	for i, s := range pages {
		pi, ci := 5+i*2, 6+i*2
		objs[pi] = fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>", pw, ph, ci+1)
		objs[ci] = fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(s), s)
	}
	off := make([]int, len(objs)+1)
	for i, x := range objs {
		off[i+1] = o.Len()
		fmt.Fprintf(&o, "%d 0 obj\n%s\nendobj\n", i+1, x)
	}
	xref := o.Len()
	fmt.Fprintf(&o, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for i := 1; i < len(off); i++ {
		fmt.Fprintf(&o, "%010d 00000 n \n", off[i])
	}
	fmt.Fprintf(&o, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return o.Bytes()
}
func esc(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	return strings.ReplaceAll(s, ")", "\\)")
}
