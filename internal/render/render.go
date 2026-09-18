package render

import (
	"bytes"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/mail"
	"net/url"
	"strings"
	"text/template"

	"golang.org/x/text/language"

	"github.com/gopherex/courier/pkg/api"
)

// ErrTemplate reports an invalid, incomplete or unrenderable template.
var (
	ErrTemplate = errors.New("invalid_template")
	// ErrConfiguration reports an incomplete or ambiguous project configuration.
	ErrConfiguration = errors.New("invalid_configuration")
)

const maxOutput = 256 * 1024

type limitedBuffer struct{ bytes.Buffer }

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	if buffer.Len()+len(value) > maxOutput {
		return 0, ErrTemplate
	}

	count, err := buffer.Buffer.Write(value)
	if err != nil {
		return count, fmt.Errorf("write rendered content: %w", err)
	}

	return count, nil
}

func execute(source string, values map[string]any, html bool) (string, error) {
	if html {
		return executeHTML(source, values)
	}

	compiled, err := template.New("notification").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", ErrTemplate
	}

	var output limitedBuffer
	if err = compiled.Execute(&output, values); err != nil {
		return "", ErrTemplate
	}

	return output.String(), nil
}

func executeHTML(source string, values map[string]any) (string, error) {
	compiled, err := htmltemplate.New("notification").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", ErrTemplate
	}

	var output limitedBuffer
	if err = compiled.Execute(&output, values); err != nil {
		return "", ErrTemplate
	}

	return output.String(), nil
}

// Notification finds one active notification key.
func Notification(config *api.ProjectConfig, key string) (*api.Notification, error) {
	for index := range config.Notifications {
		notification := &config.Notifications[index]
		if notification.Key == key && notification.Active {
			return notification, nil
		}
	}

	return nil, ErrConfiguration
}

// Channel finds a channel configured for the notification.
func Channel(notification *api.Notification, channel api.Channel) (*api.ChannelConfig, error) {
	for index := range notification.Channels {
		config := &notification.Channels[index]
		if config.Channel == channel {
			return config, nil
		}
	}

	return nil, ErrConfiguration
}

func selectTemplate(config *api.ChannelConfig, locale, fallback string) (*api.Template, error) {
	if locale == "" {
		locale = fallback
	}

	tag, err := language.Parse(locale)
	if err != nil {
		return nil, ErrConfiguration
	}

	base, _ := tag.Base()
	for _, candidate := range []string{tag.String(), base.String(), fallback} {
		for index := range config.Templates {
			item := &config.Templates[index]
			if strings.EqualFold(item.Locale, candidate) {
				return item, nil
			}
		}
	}

	return nil, ErrTemplate
}

// Render uses the same strict renderer for preview, test sends and delivery admission.
func Render(config *api.ChannelConfig, locale, fallback string, data api.Data) (*api.Rendered, error) {
	engine, err := Compile(config.Schema)
	if err != nil {
		return nil, err
	}

	values, err := Values(data)
	if err != nil {
		return nil, err
	}

	if len(engine.Validate(values).GetErrors()) != 0 {
		return nil, ErrData
	}

	selected, err := selectTemplate(config, locale, fallback)
	if err != nil {
		return nil, err
	}

	result := &api.Rendered{Locale: selected.Locale}

	fields := []struct {
		source      string
		destination *string
		html        bool
	}{
		{selected.Subject.Value, &result.Subject, false},
		{selected.Text.Value, &result.Text, false},
		{selected.HTML.Value, &result.HTML, true},
		{selected.ReplyTo.Value, &result.ReplyTo, false},
		{selected.Title.Value, &result.Title, false},
		{selected.Body.Value, &result.Body, false},
		{selected.URL.Value, &result.URL, false},
	}
	for _, field := range fields {
		*field.destination, err = execute(field.source, values, field.html)
		if err != nil {
			return nil, err
		}
	}

	if validationErr := validateRendered(config.Channel, result); validationErr != nil {
		return nil, validationErr
	}

	return result, nil
}

func validateRendered(channel api.Channel, result *api.Rendered) error {
	if strings.ContainsAny(result.Subject+result.ReplyTo, "\r\n") {
		return ErrTemplate
	}

	if result.ReplyTo != "" {
		if _, err := mail.ParseAddress(result.ReplyTo); err != nil {
			return ErrTemplate
		}
	}

	if channel == api.ChannelEmail {
		if result.Subject == "" || (result.Text == "" && result.HTML == "") {
			return ErrTemplate
		}
	} else if result.Title == "" || result.Body == "" {
		return ErrTemplate
	}

	if result.URL != "" {
		target, err := url.Parse(result.URL)
		if err != nil || target.Scheme != "https" || target.Hostname() == "" {
			return ErrTemplate
		}
	}

	return nil
}

// ValidateProject checks identifiers, localization, schemas and template syntax before saving.
func ValidateProject(config *api.ProjectConfig) error {
	locale, err := language.Parse(config.DefaultLocale)
	if err != nil || locale.String() != config.DefaultLocale {
		return ErrConfiguration
	}

	keys := make(map[string]bool)
	for _, notification := range config.Notifications {
		if keys[notification.Key] {
			return ErrConfiguration
		}

		keys[notification.Key] = true

		channels := make(map[api.Channel]bool)
		for _, channel := range notification.Channels {
			if channels[channel.Channel] {
				return ErrConfiguration
			}

			channels[channel.Channel] = true
			if _, err = Compile(channel.Schema); err != nil {
				return err
			}

			if validationErr := validateTemplates(&channel, config.DefaultLocale); validationErr != nil {
				return validationErr
			}
		}
	}

	return nil
}

func validateTemplates(config *api.ChannelConfig, fallback string) error {
	locales := make(map[string]bool)

	for index := range config.Templates {
		item := &config.Templates[index]

		tag, err := language.Parse(item.Locale)
		if err != nil || tag.String() != item.Locale || locales[item.Locale] {
			return ErrConfiguration
		}

		locales[item.Locale] = true

		sources := []string{
			item.Subject.Value,
			item.Text.Value,
			item.ReplyTo.Value,
			item.Title.Value,
			item.Body.Value,
			item.URL.Value,
		}
		for _, source := range sources {
			if _, err = template.New("notification").Option("missingkey=error").Parse(source); err != nil {
				return ErrTemplate
			}
		}

		if _, err = htmltemplate.New("notification").Option("missingkey=error").Parse(item.HTML.Value); err != nil {
			return ErrTemplate
		}

		if config.Channel == api.ChannelEmail {
			if item.Subject.Value == "" || (item.Text.Value == "" && item.HTML.Value == "") {
				return ErrTemplate
			}
		} else if item.Title.Value == "" || item.Body.Value == "" {
			return ErrTemplate
		}
	}

	if !locales[fallback] {
		return ErrConfiguration
	}

	return nil
}
