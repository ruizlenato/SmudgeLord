package utils

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var htmlTagRegex = regexp.MustCompile(`<[^>]+>`)

func StripTags(s string) string {
	return htmlTagRegex.ReplaceAllString(s, "")
}

func RichHTML(s string) string {
	return "<p>" + strings.ReplaceAll(s, "\n", "<br/>") + "</p>"
}

func RichMessageButton(label, style, buttonType, attr, value string) string {
	var attrs []string
	if style != "" {
		attrs = append(attrs, fmt.Sprintf(`style=%q`, style))
	}
	attrs = append(attrs, fmt.Sprintf(`%s=%q`, attr, html.EscapeString(value)))
	return fmt.Sprintf("<tg-button type=%q %s>%s</tg-button>\n",
		buttonType, strings.Join(attrs, " "), html.EscapeString(label))
}

func RichMessageButtonRow(buttons ...string) string {
	var sb strings.Builder
	sb.WriteString("<tg-button-row>\n")
	for _, button := range buttons {
		sb.WriteString("\t")
		sb.WriteString(button)
	}
	sb.WriteString("</tg-button-row>\n")
	return sb.String()
}
