package delivery

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"net/mail"
	"net/url"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/fcm/v1"

	"github.com/gopherex/courier/pkg/api"
)

// Validate verifies provider settings without sending traffic.
func (sender *Sender) Validate(settings *api.ProviderSettings) error {
	if settings.SMTP.Set != (settings.Channel == api.ChannelEmail) ||
		settings.Webpush.Set != (settings.Channel == api.ChannelWebpush) ||
		settings.Fcm.Set != (settings.Channel == api.ChannelFcm) {
		return ErrSettings
	}

	switch settings.Channel {
	case api.ChannelEmail:
		return sender.validateSMTP(&settings.SMTP.Value)
	case api.ChannelWebpush:
		return validateWebPush(&settings.Webpush.Value)
	case api.ChannelFcm:
		return validateFCM(&settings.Fcm.Value)
	default:
		return ErrSettings
	}
}

func (sender *Sender) validateSMTP(settings *api.SMTPSettings) error {
	if strings.ContainsAny(settings.Host+settings.From, "\r\n") || strings.ContainsAny(settings.Host, "/:@ ") {
		return ErrSettings
	}

	if settings.TLS == api.SMTPSettingsTLSPlain && !sender.AllowPlainSMTP {
		return ErrSettings
	}

	if _, err := mail.ParseAddress(settings.From); err != nil {
		return ErrSettings
	}

	return nil
}

func validateWebPush(config *api.WebPushSettings) error {
	public, err := base64.RawURLEncoding.DecodeString(config.PublicKey)
	if err != nil {
		return ErrSettings
	}

	private, err := base64.RawURLEncoding.DecodeString(config.PrivateKey)
	if err != nil {
		return ErrSettings
	}

	key, err := ecdh.P256().NewPrivateKey(private)
	if err != nil {
		return ErrSettings
	}

	publicKey, err := ecdh.P256().NewPublicKey(public)
	if err != nil || !key.PublicKey().Equal(publicKey) {
		return ErrSettings
	}

	subject, err := url.Parse(config.Subject)
	if err != nil || (subject.Scheme != "mailto" && subject.Scheme != "https") {
		return ErrSettings
	}

	return nil
}

func validateFCM(config *api.FCMSettings) error {
	raw, err := json.Marshal(config.ServiceAccount)
	if err != nil {
		return ErrSettings
	}

	credentials, err := google.JWTConfigFromJSON(raw, fcm.FirebaseMessagingScope)
	if err != nil || credentials.Email == "" || credentials.TokenURL != "https://oauth2.googleapis.com/token" {
		return ErrSettings
	}

	return nil
}

// ValidateTarget enforces one address matching the explicitly selected channel.
func ValidateTarget(input *api.DeliveryInput) error {
	if input.Email.Set != (input.Channel == api.ChannelEmail) ||
		input.Webpush.Set != (input.Channel == api.ChannelWebpush) ||
		input.Fcm.Set != (input.Channel == api.ChannelFcm) {
		return ErrSettings
	}

	switch input.Channel {
	case api.ChannelEmail:
		return validateEmail(input.Email.Value.Address)
	case api.ChannelWebpush:
		return validateSubscription(&input.Webpush.Value)
	case api.ChannelFcm:
		if input.Fcm.Value.Token == "" {
			return ErrSettings
		}

		return nil
	default:
		return ErrSettings
	}
}

func validateEmail(value string) error {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || strings.ContainsAny(value, "\r\n") {
		return ErrSettings
	}

	return nil
}

func validateSubscription(target *api.WebPushTarget) error {
	if target.Endpoint.Scheme != "https" || target.Endpoint.Hostname() == "" || target.Endpoint.User != nil {
		return ErrSettings
	}

	public, err := base64.RawURLEncoding.DecodeString(target.P256dh)
	if err != nil {
		return ErrSettings
	}

	if _, err = ecdh.P256().NewPublicKey(public); err != nil {
		return ErrSettings
	}

	auth, err := base64.RawURLEncoding.DecodeString(target.Auth)

	const authLength = 16
	if err != nil || len(auth) != authLength {
		return ErrSettings
	}

	return nil
}
