package catalogsync

import (
	"bufio"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// sv42Row is one SV 4.2 record (columns 1–11 of the KBA layout).
type sv42Row struct {
	HSN, TSN, Manufacturer, Brand, TradeName string
	Year                                     *int64
	Class, Body, FuelCode                    string
	PowerKW, DisplacementCC                  *int64
}

var (
	sv42NumbersRe = regexp.MustCompile(`^\s*1\s+2\s+3\s+4\s+5\s+6\s+7\s+8\s+9\s+10\s+11\b`)
	sv42RecordRe  = regexp.MustCompile(`^\s*\d{4}\s+[0-9A-Z]{3}\s`)
	sv42DateRe    = regexp.MustCompile(`\b\d{2}\.\d{2}\.(\d{4})\b`)
)

// readSV42 parses `pdftotext -layout` output of the SV 4.2 PDF. Column
// boundaries come from the repeated column-number line ("1 2 3 … 15") under
// each page header, never from hard-coded offsets (research C7).
func readSV42(f io.Reader, rep *Report) ([]sv42Row, error) {
	var starts []int
	var rows []sv42Row
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := []rune(sc.Text())
		if sv42NumbersRe.MatchString(string(line)) {
			starts = columnCenters(line)
			continue
		}
		if starts == nil || !sv42RecordRe.MatchString(string(line)) {
			continue
		}
		cols := splitColumns(line, starts)
		r := sv42Row{
			HSN: cols[0], TSN: strings.ToUpper(firstN(cols[1], 3)), Manufacturer: cols[2], Brand: cols[3], TradeName: cols[4],
			Class: cols[6], Body: cols[7], FuelCode: cols[8], PowerKW: atoiPtr(cols[9]), DisplacementCC: atoiPtr(cols[10]),
		}
		if m := sv42DateRe.FindStringSubmatch(cols[5]); m != nil {
			r.Year = atoiPtr(m[1])
		}
		if r.Manufacturer == "" || r.TradeName == "" {
			rep.rejected++
			continue
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if starts == nil {
		return nil, errors.New("sv42: column-number header line not found (expected pdftotext -layout output)")
	}
	return rows, nil
}

// columnCenters returns the rune position of each number on the column
// header line ("1 2 3 … 15").
func columnCenters(line []rune) []int {
	var centers []int
	for i := 0; i < len(line); {
		if line[i] >= '0' && line[i] <= '9' {
			j := i
			for j < len(line) && line[j] >= '0' && line[j] <= '9' {
				j++
			}
			centers = append(centers, (i+j-1)/2)
			i = j
			continue
		}
		i++
	}
	return centers
}

var fieldRe = regexp.MustCompile(`\S+(?: \S+)*`)

// splitColumns assigns every field (text separated by two or more spaces) to
// the column whose header number is closest to the field's centre, so the
// split works whether pdftotext aligns a column left, right or centred.
func splitColumns(line []rune, centers []int) []string {
	var cols [11]string
	s := string(line)
	for _, loc := range fieldRe.FindAllStringIndex(s, -1) {
		from := len([]rune(s[:loc[0]]))
		to := from + len([]rune(s[loc[0]:loc[1]])) - 1
		mid := (from + to) / 2
		best, dist := -1, 1<<30
		for k, c := range centers {
			if d := abs(c - mid); d < dist {
				best, dist = k, d
			}
		}
		if best < 0 || best >= len(cols) {
			continue
		}
		cols[best] = strings.TrimSpace(cols[best] + " " + s[loc[0]:loc[1]]) //nolint:gosec // best is bounds-checked above
	}
	return cols[:]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func atoiPtr(s string) *int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(s), ".", ""), 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
