package db

import (
	"testing"
)

func TestIsOverlapping(t *testing.T) {
	p1 := panel{"1", "visualization", 1, 1, 4, 3, nil, nil}
	p2 := panel{"2", "search", 5, 1, 4, 4, nil, nil}
	if p1.intersects(p2) {
		t.Errorf("p1 is not overlapping p2:\ngot %t\nwant %t", true, false)
	}
	p := panel{"1", "visualization", 1, 1, 4, 3, nil, nil}
	if !p1.intersects(p) {
		t.Errorf("p1 is overlapping p2:\ngot %t\nwant %t", false, true)
	}
	p = panel{"1", "visualization", 4, 1, 4, 3, nil, nil}
	if !p1.intersects(p) {
		t.Errorf("p1 is overlapping p2:\ngot %t\nwant %t", false, true)
	}
	p = panel{"1", "visualization", 5, 1, 4, 3, nil, nil}
	if p1.intersects(p) {
		t.Errorf("p1 is overlapping p2:\ngot %t\nwant %t", true, false)
	}
	p = panel{"1", "visualization", 1, 1, 6, 3, nil, nil}
	if !p1.intersects(p) {
		t.Errorf("p1 is overlapping p2:\ngot %t\nwant %t", false, true)
	}
}

func TestFittablePos(t *testing.T) {
	/*
			 123456789012
		    1111122223336
		    2111122223336
		    3111122223336
		    4444422223336
		    54444555555X6
		    64444555555X6
		    7XXXX555555X6
		    877777
		    9
	*/
	panels := []panel{
		panel{"1", "visualization", 1, 1, 4, 3, nil, nil},
		panel{"2", "search       ", 5, 1, 4, 4, nil, nil},
		panel{"3", "visualization", 9, 1, 3, 4, nil, nil},
		panel{"4", "visualization", 1, 4, 4, 3, nil, nil},
	}
	p := panel{"1", "visualization", 1, 1, 4, 3, nil, nil}
	if x, y := fittablePos(panels, p); x != -1 || y != -1 {
		t.Errorf("panel already exists:\ngot  %d, %d\nwant %d, %d", x, y, -1, -1)
	}
	p = panel{"5", "visualization", 1, 1, 6, 3, nil, nil}
	if x, y := fittablePos(panels, p); x != 5 || y != 5 {
		t.Errorf("non-expected position:\ngot  %d, %d\nwant %d, %d", x, y, 5, 5)
	} else {
		p.Col = x
		p.Row = y
	}
	panels = append(panels, p)
	p = panel{"6", "visualization", 1, 1, 1, 7, nil, nil}
	if x, y := fittablePos(panels, p); x != 12 || y != 1 {
		t.Errorf("non-expected position:\ngot  %d, %d\nwant %d, %d", x, y, 12, 1)
	} else {
		p.Col = x
		p.Row = y
	}
	panels = append(panels, p)
	p = panel{"7", "visualization", 1, 1, 5, 1, nil, nil}
	if x, y := fittablePos(panels, p); x != 1 || y != 8 {
		t.Errorf("non-expected position:\ngot  %d, %d\nwant %d, %d", x, y, 1, 8)
	} else {
		p.Col = x
		p.Row = y
	}
}
