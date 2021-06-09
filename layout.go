package ortfomk

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	mapset "github.com/deckarep/golang-set"
	ortfodb "github.com/ortfo/db"
)

// LayedOutCell represents a cell of a built layout
type LayedOutCell struct {
	GridArea GridArea
	// Convenience content type, first part of content type except
	// for application/, where application/pdf becomes pdf and (maybe) others
	GeneralContentType string
	// The three possible cells
	ortfodb.Media
	ortfodb.Paragraph
	ortfodb.Link
	// Metadata from the work
	Metadata *WorkMetadata
}

type Layout [][]LayedOutCell

// GridArea represents the grid area of a cell
type GridArea struct {
	Type  string
	Index int
}

func (g GridArea) String() string {
	if g.Index == 0 {
		return g.Type
	}
	return fmt.Sprintf("%s%d", g.Type, g.Index)
}

// ID Returns a layed out cell's ID, removing ambiguity
// (since a cell cannot be two things at the same time, .ID will be .Paragraph.ID for a paragraph, etc.)
func (l LayedOutCell) ID() string {
	if l.GridArea.Type == "p" {
		return l.Paragraph.ID
	}
	if l.GridArea.Type == "m" {
		return l.Media.ID
	}
	if l.GridArea.Type == "l" {
		return l.Link.ID
	}
	return ""
}

// Title Returns a layed out cell's Title, removing ambiguity
// (since a cell cannot be two things at the same time, .Title will be .Media.Title for a media, etc.)
func (l LayedOutCell) Title() string {
	if l.GridArea.Type == "p" {
		return ""
	}
	if l.GridArea.Type == "m" {
		return l.Media.Title
	}
	if l.GridArea.Type == "l" {
		return l.Link.Title
	}
	return ""
}

func (l LayedOutCell) String() string {
	return fmt.Sprintf("<%s id=%s index=%d>", l.GridArea.Type, l.ID(), l.GridArea.Index)
}

// Type is a shortcut to GridArea.Type
func (l LayedOutCell) Type() string {
	return l.GridArea.Type
}

// CSSGridTemplateAreas returns a css grid-template-areas-compatible string value
func CSSGridTemplateAreas(layout Layout) (repr string) {
	var longestRowLen int
	for _, row := range layout {
		if len(row) > longestRowLen {
			longestRowLen = len(row)
		}
	}
	for _, row := range layout {
		rowRepr := ""
		for _, element := range repeatToLen(row, longestRowLen) {
			rowRepr += element.GridArea.String() + " "
		}
		repr += fmt.Sprintf("%q ", strings.TrimSpace(rowRepr))
	}
	// Add row for <h1> (TODO: find a better way to do this)
	h1repr := strings.ReplaceAll(fmt.Sprintf("%q ", strings.Repeat("h1 ", longestRowLen)), ` "`, `"`)
	return strings.TrimSpace(h1repr + repr)
}

// repeatToLen repeats elements in the given slice until the resulting slice has a length of targetLen.
// repeatToLen panics if len(s) > targetLen.
func repeatToLen(s []LayedOutCell, targetLen int) []LayedOutCell {
	if len(s) > targetLen {
		panic(fmt.Errorf("given slice is lengthier than targetLen (has length %d > %d)", len(s), targetLen))
	}
	// TODO: more intelligent one that distributes elements in an equal fashion. Right now it's just taking the last element
	for i := len(s); i < targetLen; i++ {
		s = append(s, s[len(s)-1])
	}
	return s
}

func buildLayoutErrorMessage(whatsMissing string, work *WorkOneLang, usedCount int, layout [][]string) string {
	return fmt.Sprintf(`not enough %s to satisfy the given layout:

	· Layout is:
	%v

	· work has only %d %s
	`, whatsMissing, layout, usedCount, whatsMissing)
}

// CSSGridTemplateAreas returns a css grid-template-areas-compatible string value
// that represents work.Metadata.Layout
func (work WorkOneLang) CSSGridTemplateAreas() (value string) {
	// FIXME: this might be (a bit) computationally expensive
	return CSSGridTemplateAreas(work.LayedOut())
}

// ProperLayout turns a an untyped layout from metadata into a [][]string,
// turning string elements into a one-element slice, so that it can be used
// in loops without type errors
func (metadata WorkMetadata) ProperLayout() (proper [][]string, err error) {
	// TODO: also support "direct css grid template areas syntax" where the value of metadata.Layout is a string
	// that could be returned by .CSSGridTemplateAreas (except for quotes, not required when parsing here.)
	for _, row := range metadata.Layout {
		rowType := reflect.TypeOf(row)
		if rowType.Kind() == reflect.String {
			// If it is a string, append a 1-element slice
			proper = append(proper, []string{row.(string)})
		} else if rowType.Kind() == reflect.Array || rowType.Kind() == reflect.Slice {
			// If it's a slice of strings, append it normally
			rowString := make([]string, 0)
			for _, element := range row.([]interface{}) {
				if element == nil {
					// null is a spacer
					rowString = append(rowString, ".")
				} else {
					rowString = append(rowString, fmt.Sprint(element))
				}
			}
			proper = append(proper, rowString)
		} else {
			spew.Dump(row, reflect.TypeOf(row))
			err = fmt.Errorf("%#v is neither a list of string(s) nor a string, it is a %s (%T)", row, rowType.Name(), row)
			return
		}
	}
	return
}

// LayedOut returns an matrix of dimension 2 of LayedOutCells
// arranaged following the work's 'layout' metadata field
func (work WorkOneLang) LayedOut() (cells Layout) {
	usedCounts := map[string]int{"p": 0, "m": 0, "l": 0}
	seenCells := mapset.NewSet()

	// Coerce the layout into a proper [][]string
	layoutString, err := work.Metadata.ProperLayout()
	if err != nil {
		panic(err)
	}
	// If it's empty, that means the layout was empty all along:
	// auto-create one.
	if len(layoutString) == 0 {
		layoutString = NewLayoutAuto(&work)
	}

	for _, rowString := range layoutString {
		row := make([]LayedOutCell, 0, len(rowString))
		for _, cellString := range rowString {
			cell, err := ParseStringCell(cellString)
			if err != nil {
				panic(err)
			}

			if seenCells.Contains(cell.GridArea.String()) {
				continue
			}

			cell.Metadata = &work.Metadata

			if cell.GridArea.Index == 0 && cell.GridArea.Type != "." {
				cell.GridArea.Index = usedCounts[cell.GridArea.Type] + 1
				// Do not increment usedCounts if the index was explicitly stated:
				// - [p, .]
				// - p3
				// - p # <-- would resolve to p3 if usedCounts was incremented in all cases!
				// TODO: not sure about this behavior, should it resolve to p3? definitely not p4 though.
				usedCounts[cell.GridArea.Type]++
			}

			switch cell.GridArea.Type {
			case ".":
				// nothing left to do, a spacer holds no data
			case "p":
				// to get to index i, the array needs to have at least i+1 elements
				// to get to index i-1, the array needs to have at least i+1-1=i elements
				// if it has _strictly less_, panic.
				if len(work.Paragraphs) < cell.GridArea.Index {
					panic(buildLayoutErrorMessage("paragraphs", &work, len(work.Paragraphs), layoutString))
				}
				cell.Paragraph = work.Paragraphs[cell.GridArea.Index-1]
			case "l":
				if len(work.Links) < cell.GridArea.Index {
					panic(buildLayoutErrorMessage("links", &work, len(work.Links), layoutString))
				}
				cell.Link = work.Links[cell.GridArea.Index-1]
			case "m":
				if len(work.Media) < cell.GridArea.Index {
					panic(buildLayoutErrorMessage("media", &work, len(work.Media), layoutString))
				}
				cell.Media = work.Media[cell.GridArea.Index-1]
				cell.GeneralContentType = strings.Split(cell.Media.ContentType, "/")[0]
				if cell.Media.ContentType == "application/pdf" {
					cell.GeneralContentType = "pdf"
				}
			default:
				printfln("\nWARN: While layouting %s: element %s has no Type", work.ID, cellString)
			}
			row = append(row, cell)
			seenCells.Add(cell.GridArea.String())
		}
		cells = append(cells, row)
	}
	return
}

func NewLayoutAuto(work *WorkOneLang) [][]string {
	layout := make([][]string, 0)
	var usedParagraphs, usedLinks, usedMedia int
	for range work.Paragraphs {
		usedParagraphs++
		layout = append(layout, []string{fmt.Sprintf("p%d", usedParagraphs)})
	}
	for range work.Media {
		usedLinks++
		layout = append(layout, []string{fmt.Sprintf("m%d", usedLinks)})
	}
	for range work.Links {
		usedMedia++
		layout = append(layout, []string{fmt.Sprintf("l%d", usedMedia)})
	}
	return layout
}

func ParseStringCell(stringCell string) (cell LayedOutCell, err error) {
	if len(stringCell) > 2 {
		return cell, fmt.Errorf("malformed layout element %#v: has more than 2 characters", stringCell)
	}
	if len(stringCell) == 1 {
		if !IsValidCellType(stringCell) {
			return cell, fmt.Errorf("malformed layout element %#v: unknown cell type", stringCell)
		}
		cell.GridArea.Type = stringCell
		return
	}
	cell.GridArea.Type = stringCell[:1]
	if !IsValidCellType(cell.GridArea.Type) {
		return cell, fmt.Errorf("malformed layout element %#v: unknown cell type", stringCell[:1])
	}
	elementIndex, err := strconv.ParseUint(stringCell[1:2], 10, 16)
	if err != nil {
		return cell, fmt.Errorf("malformed layout element %#v: element index %#v is not an integer", stringCell, stringCell[1:2])
	}
	cell.GridArea.Index = int(elementIndex)
	return
}

func IsValidCellType(cellType string) bool {
	return len(cellType) == 1 && strings.Contains("lmp.", cellType)
}
