package mocka

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/castingcode/mocaprotocol"
)

// csvColumnTypes are the column types a CSV result file may declare.
var csvColumnTypes = []mocaprotocol.MocaDataType{
	mocaprotocol.MocaString,
	mocaprotocol.MocaInt,
	mocaprotocol.MocaFloat,
	mocaprotocol.MocaDate,
	mocaprotocol.MocaFlag,
	mocaprotocol.MocaBase64,
	mocaprotocol.MocaUnknownType,
}

// csvResultSet converts a CSV result file into a <moca-results> XML fragment.
//
// The header row declares the columns, each as name, name:TYPE, or
// name:TYPE:LENGTH. TYPE defaults to S and LENGTH to 0. Every later row is one
// data row with one value per column; an empty value is a null field.
func csvResultSet(data []byte) (string, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\uFEFF")))) // drop a UTF-8 BOM, as Excel writes
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return "", fmt.Errorf("missing header row")
	}
	if err != nil {
		return "", fmt.Errorf("reading header row: %w", err)
	}

	var results mocaprotocol.MocaResults
	for i, cell := range header {
		col, err := csvColumn(strings.TrimSpace(cell))
		if err != nil {
			return "", fmt.Errorf("header column %d: %w", i+1, err)
		}
		results.Metadata.Columns = append(results.Metadata.Columns, col)
	}

	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading row: %w", err)
		}
		line, _ := r.FieldPos(0)
		var row mocaprotocol.Row
		for i, value := range record {
			col := results.Metadata.Columns[i]
			if value == "" {
				row.Fields = append(row.Fields, mocaprotocol.Field{Null: "true"})
				continue
			}
			if err := checkCSVValue(col.Type, value); err != nil {
				return "", fmt.Errorf("line %d, column %s: %w", line, col.Name, err)
			}
			row.Fields = append(row.Fields, mocaprotocol.Field{Value: value})
		}
		results.Data.Rows = append(results.Data.Rows, row)
	}

	out, err := xml.Marshal(struct {
		XMLName xml.Name `xml:"moca-results"`
		mocaprotocol.MocaResults
	}{MocaResults: results})
	if err != nil {
		return "", fmt.Errorf("encoding results XML: %w", err)
	}
	return string(out), nil
}

// csvColumn parses a header cell of the form name[:TYPE[:LENGTH]].
func csvColumn(cell string) (mocaprotocol.Column, error) {
	parts := strings.Split(cell, ":")
	if len(parts) > 3 {
		return mocaprotocol.Column{}, fmt.Errorf("%q has too many parts (want name, name:TYPE, or name:TYPE:LENGTH)", cell)
	}
	col := mocaprotocol.Column{Name: parts[0], Type: mocaprotocol.MocaString, Length: "0", Nullable: "true"}
	if col.Name == "" {
		return mocaprotocol.Column{}, fmt.Errorf("%q has no column name", cell)
	}
	if len(parts) > 1 {
		col.Type = mocaprotocol.MocaDataType(strings.ToUpper(parts[1]))
		if !slices.Contains(csvColumnTypes, col.Type) {
			return mocaprotocol.Column{}, fmt.Errorf("%q has unknown type %q (want one of S, I, F, D, O, V, U)", cell, parts[1])
		}
	}
	if len(parts) > 2 {
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return mocaprotocol.Column{}, fmt.Errorf("%q has a length that is not a number", cell)
		}
		col.Length = parts[2]
	}
	return col, nil
}

// checkCSVValue rejects values that cannot belong to a numeric column.
func checkCSVValue(typ mocaprotocol.MocaDataType, value string) error {
	switch typ {
	case mocaprotocol.MocaInt:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("%q is not an integer", value)
		}
	case mocaprotocol.MocaFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("%q is not a number", value)
		}
	}
	return nil
}
