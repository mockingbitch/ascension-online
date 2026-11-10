package httpapi

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/chat"
	"tiendao/game/internal/lecture/hall"
)

// jsonFields lists the JSON names of a struct's exported fields.
func jsonFields(v any) []string {
	t := reflect.TypeOf(v)
	var out []string
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Contract (decisions 2026-11-10, hall shards): the Go JSON shapes of the hall and hall chat match openapi.yaml
// property for property — a field added on one side only fails here.
func TestHallContract(t *testing.T) {
	b, err := os.ReadFile("../../../../packages/contracts/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for schema, v := range map[string]any{
		"LectureState": hall.State{}, "LectureMe": hall.Me{}, "Listener": hall.Listener{}, "HallCount": hall.HallCount{},
		"ChatMessage": chat.Message{},
	} {
		s, ok := doc.Components.Schemas[schema]
		if !ok {
			t.Fatalf("openapi has no %s", schema)
		}
		var props []string
		for k := range s.Properties {
			props = append(props, k)
		}
		slices.Sort(props)
		if got := jsonFields(v); !slices.Equal(got, props) {
			t.Errorf("%s: Go %v ≠ openapi %v", schema, got, props)
		}
	}
}
