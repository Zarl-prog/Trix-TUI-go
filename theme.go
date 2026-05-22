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
	Background:    "#1e2132",
	Surface:       "#252840",
	SurfaceAlt:    "#1a1d2e",
	Border:        "#2d3154",
	BorderFocused: "#4ec9b0",
	Text:          "#cdd3de",
	TextMuted:     "#546178",
	Accent:        "#4ec9b0",
	AccentAlt:     "#3aab96",
	Success:       "#4ec9b0",
	Warning:       "#ce9178",
	Error:         "#f44747",
	CursorLine:    "#252840",
	LineNumber:    "#3d4466",
	Scrollbar:     "#2d3154",
	ScrollbarThumb:"#4ec9b0",
	Selection:     "#2d3154",
}

var Themes = []Theme{AyuDark, SeaGreen}
