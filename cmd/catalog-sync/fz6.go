package main

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// fz6Row is one line of KBA FZ 6.1 (stock by manufacturer and type).
type fz6Row struct {
	HSN, Manufacturer, TSN, TradeName string
	Count                             int64
}

type xlsxSST struct {
	Items []struct {
		T    string `xml:"t"`
		Runs []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type xlsxSheet struct {
	Rows []struct {
		Cells []struct {
			Ref    string `xml:"r,attr"`
			Type   string `xml:"t,attr"`
			Value  string `xml:"v"`
			Inline struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

// readFZ6 reads the FZ 6 workbook with the standard library only (research
// C7). The data sheet is the one whose header row names the HSN, TSN,
// manufacturer and trade-name columns.
func readFZ6(path string) ([]fz6Row, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("fz6: %w", err)
	}
	defer func() { _ = zr.Close() }()
	var sst []string
	var sheets []*zip.File
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			var s xlsxSST
			if err := decodeZipXML(f, &s); err != nil {
				return nil, err
			}
			for _, it := range s.Items {
				t := it.T
				for _, r := range it.Runs {
					t += r.T
				}
				sst = append(sst, t)
			}
		case strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml"):
			sheets = append(sheets, f)
		}
	}
	for _, f := range sheets {
		var sh xlsxSheet
		if err := decodeZipXML(f, &sh); err != nil {
			return nil, err
		}
		if rows, ok := parseFZ6Sheet(sh, sst); ok {
			return rows, nil
		}
	}
	return nil, errors.New("fz6: no sheet with HSN/TSN/Handelsname columns found")
}

func decodeZipXML(f *zip.File, v any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	return xml.NewDecoder(io.LimitReader(rc, 512<<20)).Decode(v)
}

func parseFZ6Sheet(sh xlsxSheet, sst []string) ([]fz6Row, bool) {
	col := map[string]int{}
	var rows []fz6Row
	for _, r := range sh.Rows {
		cells := map[int]string{}
		for _, c := range r.Cells {
			v := c.Value
			switch c.Type {
			case "s":
				if i, err := strconv.Atoi(v); err == nil && i >= 0 && i < len(sst) {
					v = sst[i]
				}
			case "inlineStr":
				v = c.Inline.T
			}
			cells[colIndex(c.Ref)] = strings.TrimSpace(v)
		}
		if len(col) == 0 {
			for i, v := range cells {
				h := strings.ToLower(strings.NewReplacer("\n", "", "-", "", " ", "").Replace(v))
				switch {
				case strings.HasPrefix(h, "herstellerschlüssel"):
					col["hsn"] = i
				case strings.HasPrefix(h, "typschlüssel"):
					col["tsn"] = i
				case strings.HasPrefix(h, "herstellerklartext"):
					col["make"] = i
				case strings.HasPrefix(h, "handelsname"):
					col["trade"] = i
				case strings.HasPrefix(h, "anzahl"):
					col["count"] = i
				}
			}
			if len(col) < 4 {
				clear(col)
			}
			continue
		}
		hsn, tsn := cells[col["hsn"]], strings.ToUpper(cells[col["tsn"]])
		if len(hsn) != 4 || len(tsn) != 3 {
			continue
		}
		n, _ := strconv.ParseInt(cells[col["count"]], 10, 64)
		rows = append(rows, fz6Row{HSN: hsn, TSN: tsn, Manufacturer: cells[col["make"]], TradeName: cells[col["trade"]], Count: n})
	}
	return rows, len(col) >= 4 && len(rows) > 0
}

// colIndex turns a cell reference like "C12" into a zero-based column.
func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
	}
	return n - 1
}
