package main

type Theme struct {
	Name           string
	Background     string
	Surface        string
	SurfaceAlt     string
	Border         string
	BorderFocused  string
	Text           string
	TextMuted      string
	Accent         string
	AccentAlt      string
	Success        string
	Warning        string
	Error          string
	CursorLine     string
	LineNumber     string
	Scrollbar      string
	ScrollbarThumb string
	Selection      string
}

var AyuDark = Theme{
	Name:           "Ayu Dark",
	Background:     "#0d1016",
	Surface:        "#1f2127",
	SurfaceAlt:     "#131721",
	Border:         "#3f4043",
	BorderFocused:  "#5ac1fe",
	Text:           "#bfbdb6",
	TextMuted:      "#4b4c4e",
	Accent:         "#5ac1fe",
	AccentAlt:      "#4ba8e0",
	Success:        "#aad84c",
	Warning:        "#feb454",
	Error:          "#ef7177",
	CursorLine:     "#1f2127",
	LineNumber:     "#4b4c4e",
	Scrollbar:      "#3f4043",
	ScrollbarThumb: "#5ac1fe",
	Selection:      "#3e4043",
}

var SeaGreen = Theme{
	Name:           "Sea Green",
	Background:     "#0d1a14",
	Surface:        "#112218",
	SurfaceAlt:     "#0a1510",
	Border:         "#1a3d2b",
	BorderFocused:  "#2ecc71",
	Text:           "#c8e6c9",
	TextMuted:      "#4a7c59",
	Accent:         "#2ecc71",
	AccentAlt:      "#27ae60",
	Success:        "#00e676",
	Warning:        "#ffeb3b",
	Error:          "#ef5350",
	CursorLine:     "#112218",
	LineNumber:     "#2d5a3d",
	Scrollbar:      "#1a3d2b",
	ScrollbarThumb: "#2ecc71",
	Selection:      "#1a3d2b",
}

var Themes = []Theme{AyuDark, SeaGreen}
