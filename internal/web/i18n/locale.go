package i18n

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed en.json
var enBytes []byte

//go:embed zh.json
var zhBytes []byte

var (
	mu        sync.RWMutex
	locales   = make(map[string]map[string]string)
	supported = []string{"en", "zh"}
)

func init() {
	loadLocale("en", enBytes)
	loadLocale("zh", zhBytes)
}

func loadLocale(lang string, data []byte) {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		panic("i18n: failed to parse " + lang + ".json: " + err.Error())
	}
	mu.Lock()
	locales[lang] = m
	mu.Unlock()
}

// Supported returns the list of supported locale codes.
func Supported() []string {
	return supported
}

// T returns the translation for the given key in the given language.
// It falls back to English if the key is not found in the requested language,
// then to the key itself if not found in English either.
func T(lang, key string) string {
	mu.RLock()
	defer mu.RUnlock()

	if m, ok := locales[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	// Fall back to English.
	if m, ok := locales["en"]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}
