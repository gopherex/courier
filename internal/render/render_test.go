package render_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/gopherex/courier/internal/render"
	"github.com/gopherex/courier/pkg/api"
)

func channel(t *testing.T) *api.ChannelConfig {
	t.Helper()

	var config api.ChannelConfig

	err := json.Unmarshal([]byte(`{
  "channel": "email",
  "schema": {
    "id": {
      "name": "registration"
    },
    "strict": true,
    "fields": [
      {
        "name": "name",
        "required": true,
        "string": {}
      }
    ]
  },
  "templates": [
    {
      "locale": "ru",
      "subject": "Привет, {{.name}}",
      "text": "{{.name}}",
      "html": "<b>{{.name}}</b>"
    },
    {
      "locale": "en",
      "subject": "Hello, {{.name}}",
      "text": "{{.name}}"
    }
  ]
}`), &config)
	if err != nil {
		t.Fatal(err)
	}

	return &config
}

func data(t *testing.T, raw string) api.Data {
	t.Helper()

	var value api.Data
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}

	return value
}

func TestRenderLocaleAndEscaping(t *testing.T) {
	t.Parallel()
	config := channel(t)

	rendered, err := render.Render(config, "ru-RU", "en", data(t, `{"name":"<script>"}`))
	if err != nil {
		t.Fatal(err)
	}

	if rendered.Locale != "ru" || rendered.HTML != "<b>&lt;script&gt;</b>" || rendered.Text != "<script>" {
		t.Fatalf("unexpected rendering: %+v", rendered)
	}

	rendered, err = render.Render(config, "de-DE", "en", data(t, `{"name":"Alex"}`))
	if err != nil || rendered.Locale != "en" || rendered.HTML != "" {
		t.Fatalf("whole-template fallback: %+v %v", rendered, err)
	}
}

func TestRenderRejectsUnknownMissingAndWrongTypes(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`{"name":"Alex","extra":1}`, `{}`, `{"name":12}`} {
		if _, err := render.Render(channel(t), "ru", "ru", data(t, input)); !errors.Is(err, render.ErrData) {
			t.Errorf("input %s: %v", input, err)
		}
	}

	config := channel(t)

	config.Templates[0].Subject = api.NewOptString("{{.absent}}")
	if _, err := render.Render(config, "ru", "ru", data(t, `{"name":"Alex"}`)); !errors.Is(err, render.ErrTemplate) {
		t.Fatal(err)
	}
}

func TestNestedSchemaPolicy(t *testing.T) {
	t.Parallel()

	for _, schema := range []string{
		`{"id":{"name":"test"},"strict":true,"coerce":true}`,
		`{
  "id": {
    "name": "test"
  },
  "strict": true,
  "defs": {
    "nested": {
      "id": {
        "name": "nested"
      },
      "strict": true,
      "fields": [
        {
          "name": "x",
          "string": {
            "default": "hidden"
          }
        }
      ]
    }
  }
}`,
		`{
  "id": {
    "name": "test"
  },
  "strict": true,
  "fields": [
    {
      "name": "x",
      "object": {
        "schema": {
          "id": {
            "name": "nested"
          },
          "strict": false
        }
      }
    }
  ]
}`,
		`{"id":{"name":"test"},"strict":true,"fields":[{"name":"x","computed":{"expr":"1"}}]}`,
	} {
		var input api.Schema
		if err := json.Unmarshal([]byte(schema), &input); err != nil {
			t.Fatal(err)
		}

		if _, err := render.Compile(input); !errors.Is(err, render.ErrSchema) {
			t.Fatalf("policy accepted %s: %v", schema, err)
		}
	}
}

func TestDataPreservesIntegerPrecision(t *testing.T) {
	t.Parallel()

	values, err := render.Values(data(t, `{"large":9007199254740993,"unsigned":18446744073709551615,"array":[1.5]}`))
	if err != nil {
		t.Fatal(err)
	}

	if values["large"] != int64(9007199254740993) || values["unsigned"] != uint64(18446744073709551615) {
		t.Fatal(values)
	}
}
