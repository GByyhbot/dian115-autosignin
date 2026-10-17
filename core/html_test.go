package core

import "testing"

func TestParseDocElementsAndText(t *testing.T) {
	html := `<html><body><div class="box"><a href="showup.php" id="link">已签到</a></div><form action="attendance.php"><input type="submit" name="submit" value="签到"></form></body></html>`
	doc := ParseDoc(html)
	// a 元素可见文本累积
	foundA := false
	for _, e := range doc.Elements {
		if e.Tag == "a" {
			foundA = true
			if CompactText(e.Text) != "已签到" {
				t.Fatalf("a text = %q", e.Text)
			}
		}
	}
	if !foundA {
		t.Fatal("anchor not tracked")
	}
	if CompactText(doc.Text) != "已签到" {
		t.Fatalf("doc text = %q", CompactText(doc.Text))
	}
}

func TestElementAttr(t *testing.T) {
	html := `<meta name="x-csrf-token" content="abc123"><input name="questionid" value="42">`
	doc := ParseDoc(html)
	meta, ok := doc.ElementAttr("meta", "name", "x-csrf-token")
	if !ok || meta["content"] != "abc123" {
		t.Fatalf("meta attrs = %v ok=%v", meta, ok)
	}
	inp, ok := doc.ElementAttr("input", "name", "questionid")
	if !ok || inp["value"] != "42" {
		t.Fatalf("input attrs = %v ok=%v", inp, ok)
	}
}

func TestDescendantsWithAncestorClass(t *testing.T) {
	html := `<table class="captcha"><tr><td><img src="/cap.php?1"></td></tr></table><img src="/other.jpg">`
	doc := ParseDoc(html)
	imgs := doc.DescendantsWithAncestorClass("img", "captcha")
	if len(imgs) != 1 || imgs[0].Attrs["src"] != "/cap.php?1" {
		t.Fatalf("descendants = %+v", imgs)
	}
}

func TestNestedParentChain(t *testing.T) {
	html := `<div class="a"><div class="b"><img src="x"></div></div>`
	doc := ParseDoc(html)
	img := doc.Elements[len(doc.Elements)-1]
	if img.Tag != "img" {
		t.Fatalf("last element = %s", img.Tag)
	}
	if img.ParentIdx < 0 {
		t.Fatal("img must have parent")
	}
	if doc.Elements[img.ParentIdx].Attrs["class"] != "b" {
		t.Fatalf("parent = %+v", doc.Elements[img.ParentIdx])
	}
}

func TestStripTags(t *testing.T) {
	got := StripTags(`<b>Hello</b> &amp; <i>world</i>`)
	if got != "Hello & world" {
		t.Fatalf("strip = %q", got)
	}
}
