package main

import "github.com/egoist/mygo/ui"

// The two line icons the object list uses, parsed once at startup. Icon
// paints them in the text color, so a selected (inverted) row inverts them
// with the name.
var (
	iconFolder = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M3.5 7.5A2.5 2.5 0 0 1 6 5h3.6l1.9 2H18a2.5 2.5 0 0 1 2.5 2.5v7A2.5 2.5 0 0 1 18 19H6a2.5 2.5 0 0 1-2.5-2.5z" stroke="#000000" stroke-width="1.6" stroke-linejoin="round"/></svg>`))
	iconFile   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M6.5 3.5h7l4.5 4.5v12.5h-11.5z" stroke="#000000" stroke-width="1.6" stroke-linejoin="round"/><path d="M13.5 3.5V8H18" stroke="#000000" stroke-width="1.6" stroke-linejoin="round"/></svg>`))
)

// logoMark is the cloud of the app icon, without the paper tile, for the
// title bar and the narrow login header.
var logoMark = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="205 271 602 454" fill="none"><g fill="#121212"><circle cx="512" cy="462" r="176"/><circle cx="352" cy="560" r="132"/><circle cx="672" cy="556" r="120"/><rect x="300" y="540" width="440" height="170" rx="85"/></g><path d="M512 470 L588 596 L544 596 L544 700 L480 700 L480 596 L436 596 Z" fill="#F1EFE7"/><rect x="736" y="286" width="48" height="48" fill="#1B3BFF"/></svg>`))

func entryIcon(folder bool) *ui.SVG {
	if folder {
		return iconFolder
	}
	return iconFile
}
