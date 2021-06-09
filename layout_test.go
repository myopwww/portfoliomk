package ortfomk

import (
	"fmt"
	"testing"
)
func TestCSSGridTemplateAreas(t *testing.T) {
	runTestTable(t, []testData{
		{
			have: CSSGridTemplateAreas(makeGrid([][]GridArea{
				{{"p", 1}}, 
				{{"p", 2}, {"l", 1}},
			})),
			want: `"h1 h1" "p1 p1" "p2 l1"`,
		},
		{
			have: CSSGridTemplateAreas(makeGrid([][]GridArea{
				{{"p", 1}, {"m", 2}}, 
				{{"l", 2}, {"m", 2}, {"l", 3}},
			})),
			want: `"h1 h1 h1" "p1 m2 m2" "l2 m2 l3"`,
		},
	})
}

func TestRepeatToLen(t *testing.T) {
	runTestTable(t, []testData{
		{
			have: fmt.Sprintf("%v", repeatToLen(makeRow([]GridArea{{"p", 1}}), 3)),
			want: `[<p id= index=1> <p id= index=1> <p id= index=1>]`,
		},
	})

	assertPanic(t, func(){
		repeatToLen(makeRow([]GridArea{{"p", 2}, {"p", 3}, {"p",1}}), 2)
	})

}

//
// layout.go-specific utils
//

func makeRow(row []GridArea) (layouted []LayedOutCell) {
	for _, gridArea := range row {
		layouted = append(layouted, LayedOutCell{GridArea: gridArea})
	}
	return
}

func makeGrid(layout [][]GridArea) (layouted [][]LayedOutCell) {
	for _, row := range layout {
		layouted = append(layouted, makeRow(row))
	}
	return
}
