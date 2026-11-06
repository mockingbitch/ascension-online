package realtime

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Templates holds announcement templates (content/story/announcements/*.yaml) keyed by event kind.
type Templates map[string][]string

// LoadTemplates reads an announcement template file.
func LoadTemplates(path string) (Templates, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Version   int       `yaml:"version"`
		Templates Templates `yaml:"templates"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("realtime: templates: %w", err)
	}
	for k, list := range f.Templates {
		if len(list) == 0 {
			return nil, fmt.Errorf("realtime: template %q has no variants", k)
		}
	}
	return f.Templates, nil
}

// Render picks a random variant of kind and substitutes {placeholders}. Unknown kind → "".
func (t Templates) Render(kind string, vars map[string]string) string {
	list := t[kind]
	if len(list) == 0 {
		return ""
	}
	s := list[rand.IntN(len(list))]
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}
