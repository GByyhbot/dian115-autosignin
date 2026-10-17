package core

import (
	"fmt"
	"testing"
)

func TestDbgParse(t *testing.T) {
	src := `<html><body><div>您已连续签到 10 天</div></body></html>`
	d := ParseDoc(src)
	fmt.Printf("TEXT=%q\n", d.Text)
	fmt.Printf("VISIBLE=%q\n", d.Visible)
	fmt.Printf("ELEMS=%d\n", len(d.Elements))
	src2 := `<input type="text"><input type="password" name="password">`
	d2 := ParseDoc(src2)
	fmt.Printf("VIS2=%q\n", d2.Visible)
	for _, e := range d2.Elements {
		fmt.Printf("ELEM %s %v\n", e.Tag, e.Attrs)
	}
}
