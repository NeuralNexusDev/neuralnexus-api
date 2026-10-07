package status

import (
	"bytes"
	"strings"

	"github.com/goccy/go-json"
)

// Color in the format of "#<hex>"
type Color string

const (
	ColorBlack       Color = "black"        // #000000
	ColorDarkBlue    Color = "dark_blue"    // #0000AA
	ColorDarkGreen   Color = "dark_green"   // #00AA00
	ColorDarkAqua    Color = "dark_aqua"    // #00AAAA
	ColorDarkRed     Color = "dark_red"     // #AA0000
	ColorDarkPurple  Color = "dark_purple"  // #AA00AA
	ColorGold        Color = "gold"         // #FFAA00
	ColorGray        Color = "gray"         // #AAAAAA
	ColorDarkGray    Color = "dark_gray"    // #555555
	ColorBlue        Color = "blue"         // #5555FF
	ColorGreen       Color = "green"        // #55FF55
	ColorAqua        Color = "aqua"         // #55FFFF
	ColorRed         Color = "red"          // #FF5555
	ColorLightPurple Color = "light_purple" // #FF55FF
	ColorYellow      Color = "yellow"       // #FFFF55
	ColorWhite       Color = "white"        // #FFFFFF
)

// ToLegacy converts the Color to its corresponding legacy formatting code.
func (c Color) ToLegacy() rune {
	// TODO: Handle hex colors by approximating
	// TODO: Note, 1.16+
	switch c {
	case ColorBlack:
		return '0'
	case ColorDarkBlue:
		return '1'
	case ColorDarkGreen:
		return '2'
	case ColorDarkAqua:
		return '3'
	case ColorDarkRed:
		return '4'
	case ColorDarkPurple:
		return '5'
	case ColorGold:
		return '6'
	case ColorGray:
		return '7'
	case ColorDarkGray:
		return '8'
	case ColorBlue:
		return '9'
	case ColorGreen:
		return 'a'
	case ColorAqua:
		return 'b'
	case ColorRed:
		return 'c'
	case ColorLightPurple:
		return 'd'
	case ColorYellow:
		return 'e'
	case ColorWhite:
		return 'f'
	default:
		return 0
	}
}

// ColorFromLegacy converts a legacy formatting code to its corresponding Color
func ColorFromLegacy(code rune) Color {
	switch code {
	case '0':
		return ColorBlack
	case '1':
		return ColorDarkBlue
	case '2':
		return ColorDarkGreen
	case '3':
		return ColorDarkAqua
	case '4':
		return ColorDarkRed
	case '5':
		return ColorDarkPurple
	case '6':
		return ColorGold
	case '7':
		return ColorGray
	case '8':
		return ColorDarkGray
	case '9':
		return ColorBlue
	case 'a', 'A':
		return ColorGreen
	case 'b', 'B':
		return ColorAqua
	case 'c', 'C':
		return ColorRed
	case 'd', 'D':
		return ColorLightPurple
	case 'e', 'E':
		return ColorYellow
	case 'f', 'F':
		return ColorWhite
	default:
		return ""
	}
}

const (
	LegacyFormatObfuscated    = "§k"
	LegacyFormatBold          = "§l"
	LegacyFormatStrikeThrough = "§m"
	LegacyFormatUnderlined    = "§n"
	LegacyFormatItalic        = "§o"
	LegacyFormatReset         = "§r"
)

// Component represents a text component
type Component struct {
	Text          string       `json:"text"`
	Obfuscated    bool         `json:"obfuscated,omitempty"`
	Bold          bool         `json:"bold,omitempty"`
	StrikeThrough bool         `json:"strikethrough,omitempty"`
	Underlined    bool         `json:"underlined,omitempty"`
	Italic        bool         `json:"italic,omitempty"`
	Color         Color        `json:"color,omitempty"`
	Extra         []*Component `json:"extra,omitempty"`
}

type component Component

func (c *Component) UnmarshalJSON(data []byte) error {
	var tmp component

	// Handle raw string case
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		tmp = component(LegacyToComponent(text))
	} else { // Handle regular JSON object case
		if err := json.Unmarshal(data, &tmp); err != nil {
			return err
		}
	}

	*c = Component(tmp)
	return nil
}

const UTF8LeadingByte = 0xC2
const SectionSign = '§'

// LegacyToComponent converts a legacy formatting code string to a Component
func LegacyToComponent(s string) Component {
	var c = &Component{}
	var currentComponent = c
	var text string
	var color, newColor Color
	var bold, italic, underlined, strikethrough, obfuscated bool
	var reset = false

	// Create a new nested child when a new colour is encountered. Create a new top-level child if there is a reset.
	for i := 0; i < len(s); i++ {
		if s[i] != UTF8LeadingByte || i+2 >= len(s) || s[i+1] != SectionSign {
			text += string(s[i])
			continue
		}
		i += 2
		code := s[i]
		newColor = ColorFromLegacy(rune(code))
		switch code {
		case 'k':
			obfuscated = true
		case 'l':
			bold = true
		case 'm':
			strikethrough = true
		case 'n':
			underlined = true
		case 'o':
			italic = true
		case 'r':
			reset = true
		}

		if newColor != "" && newColor != color {
			hadFormat := bold || italic || underlined || strikethrough || obfuscated

			if text != "" {
				newComponent := &Component{
					Text:          text,
					Color:         color,
					Bold:          bold,
					Italic:        italic,
					Underlined:    underlined,
					StrikeThrough: strikethrough,
					Obfuscated:    obfuscated,
				}
				currentComponent.Extra = append(currentComponent.Extra, newComponent)
				if !hadFormat {
					currentComponent = newComponent
				}
			}

			if hadFormat {
				currentComponent = c
			}

			text = ""
			color = newColor
			bold, italic, underlined, strikethrough, obfuscated = false, false, false, false, false
		}

		if reset {
			if text != "" {
				newComponent := &Component{
					Text:          text,
					Color:         color,
					Bold:          bold,
					Italic:        italic,
					Underlined:    underlined,
					StrikeThrough: strikethrough,
					Obfuscated:    obfuscated,
				}
				currentComponent.Extra = append(currentComponent.Extra, newComponent)
			}
			currentComponent = c

			text = ""
			color = ""
			bold, italic, underlined, strikethrough, obfuscated = false, false, false, false, false

			reset = false
		}
	}

	// Add the last component if there's any text left
	if text != "" {
		c.Extra = append(c.Extra, &Component{
			Text:          text,
			Color:         color,
			Bold:          bold,
			Italic:        italic,
			Underlined:    underlined,
			StrikeThrough: strikethrough,
			Obfuscated:    obfuscated,
		})
	}

	return *c
}

func (c *Component) String() string {
	var buf bytes.Buffer
	buf.WriteString(c.Text)
	for _, extra := range c.Extra {
		buf.WriteString(extra.String())
	}
	return buf.String()
}

// ToLegacy converts the Component to the legacy formatting code string
func (c *Component) ToLegacy() string {
	var b = strings.Builder{}

	// Convert color to legacy formatting code
	if c.Color != "" {
		b.WriteRune(SectionSign)
		b.WriteRune(c.Color.ToLegacy())
	}

	// Convert formatting to legacy formatting codes
	if c.Bold {
		b.WriteString(LegacyFormatBold)
	}
	if c.Italic {
		b.WriteString(LegacyFormatItalic)
	}
	if c.Underlined {
		b.WriteString(LegacyFormatUnderlined)
	}
	if c.StrikeThrough {
		b.WriteString(LegacyFormatStrikeThrough)
	}
	if c.Obfuscated {
		b.WriteString(LegacyFormatObfuscated)
	}

	// Parse child components
	b.WriteString(c.Text)
	for _, extra := range c.Extra {
		b.WriteString(extra.ToLegacy())
	}

	// Add reset code after child components
	b.WriteString(LegacyFormatReset)

	return b.String()
}
