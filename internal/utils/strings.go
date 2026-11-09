package utils

import (
	"encoding/base64"
	"math/rand"
	"regexp"
	"strings"
)

func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result strings.Builder
	for range n {
		result.WriteByte(letters[rand.Intn(len(letters))])
	}
	return result.String()
}

func EscapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

var allowedTelegramHTMLTag = regexp.MustCompile(`^</?(?:b|i|u|s|code|blockquote|a|tg-emoji)(?:\s+[^>]*)?>$`)

func telegramHTMLTagName(tag string) string {
	start := 1
	if len(tag) > 2 && tag[1] == '/' {
		start = 2
	}

	end := start
	for end < len(tag) {
		c := tag[end]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			end++
			continue
		}
		break
	}

	if start >= end {
		return ""
	}

	return strings.ToLower(tag[start:end])
}

func SanitizeTelegramHTML(text string) string {
	if text == "" {
		return text
	}

	var builder strings.Builder
	builder.Grow(len(text))
	openTags := make([]string, 0, 8)

	for i := 0; i < len(text); {
		if text[i] != '<' {
			builder.WriteByte(text[i])
			i++
			continue
		}

		if i+1 >= len(text) {
			builder.WriteString("&lt;")
			i++
			continue
		}

		next := text[i+1]
		isOpeningTagStart := (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z')
		isClosingTagStart := next == '/' && i+2 < len(text) && ((text[i+2] >= 'a' && text[i+2] <= 'z') || (text[i+2] >= 'A' && text[i+2] <= 'Z'))
		if !isOpeningTagStart && !isClosingTagStart {
			builder.WriteString("&lt;")
			i++
			continue
		}

		end := strings.IndexByte(text[i:], '>')
		if end == -1 {
			builder.WriteString("&lt;")
			i++
			continue
		}

		end += i
		tag := text[i : end+1]
		if allowedTelegramHTMLTag.MatchString(tag) {
			isClosing := len(tag) > 2 && tag[1] == '/'
			tagName := telegramHTMLTagName(tag)
			if !isClosing {
				builder.WriteString(tag)
				openTags = append(openTags, tagName)
			} else {
				matchIndex := -1
				for j := len(openTags) - 1; j >= 0; j-- {
					if openTags[j] == tagName {
						matchIndex = j
						break
					}
				}

				if matchIndex == -1 {
					builder.WriteString(EscapeHTML(tag))
				} else {
					for j := len(openTags) - 1; j > matchIndex; j-- {
						builder.WriteString("</")
						builder.WriteString(openTags[j])
						builder.WriteString(">")
					}

					builder.WriteString(tag)
					openTags = openTags[:matchIndex]
				}
			}
		} else {
			builder.WriteString(EscapeHTML(tag))
		}

		i = end + 1
	}

	for j := len(openTags) - 1; j >= 0; j-- {
		builder.WriteString("</")
		builder.WriteString(openTags[j])
		builder.WriteString(">")
	}

	return builder.String()
}

func SanitizeString(input string) string {
	illegalChars := []rune{'}', '{', '%', '>', '<', '^', ';', ':', '`', '$', '"', '@', '=', '?', '|', '*'}

	result := strings.ReplaceAll(input, "/", "_")
	result = strings.ReplaceAll(result, "\\", "_")

	for _, char := range illegalChars {
		result = strings.ReplaceAll(result, string(char), "")
	}

	return result
}

func FileTypeByFileID(fileID string) int32 {
	if fileID == "" {
		return 0
	}

	fileID = strings.ReplaceAll(fileID, "-", "+")
	fileID = strings.ReplaceAll(fileID, "_", "/")

	decoded, err := base64.RawStdEncoding.DecodeString(fileID)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(fileID)
		if err != nil {
			return 0
		}
	}

	if len(decoded) < 4 {
		return 0
	}

	fileType := int32(decoded[0]) | int32(decoded[1])<<8 | int32(decoded[2])<<16 | int32(decoded[3])<<24
	fileType = fileType & 0xFF

	return fileType
}
