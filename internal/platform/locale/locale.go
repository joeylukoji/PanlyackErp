package locale

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/url"
	"strings"
	"unicode"
)

type contextKey struct{}
type preference struct{ language, returnPath string }

//go:embed en.json
var englishJSON []byte

var english = loadCatalog()

func loadCatalog() map[string]string {
	var result map[string]string
	if err := json.Unmarshal(englishJSON, &result); err != nil {
		panic("invalid embedded English translation catalog: " + err.Error())
	}
	return result
}

func WithLanguage(ctx context.Context, language, returnPath string) context.Context {
	if language != "en" {
		language = "fr"
	}
	return context.WithValue(ctx, contextKey{}, preference{language: language, returnPath: returnPath})
}

func Language(ctx context.Context) string {
	value, ok := ctx.Value(contextKey{}).(preference)
	if !ok {
		return "fr"
	}
	return value.language
}

// Text translates controlled interface copy only. Never pass customer or product data.
func Text(ctx context.Context, source string) string {
	if Language(ctx) != "en" {
		return source
	}
	key := strings.Join(strings.Fields(source), " ")
	translated, ok := english[key]
	if !ok {
		return source
	}
	left := source[:len(source)-len(strings.TrimLeftFunc(source, unicode.IsSpace))]
	right := source[len(strings.TrimRightFunc(source, unicode.IsSpace)):]
	return left + translated + right
}

func SwitchURL(ctx context.Context, language string) string {
	value, _ := ctx.Value(contextKey{}).(preference)
	if value.returnPath == "" {
		value.returnPath = "/"
	}
	return "/language?lang=" + url.QueryEscape(language) + "&return_to=" + url.QueryEscape(value.returnPath)
}
