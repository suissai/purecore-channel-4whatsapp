package geoloc2web

import (
	"fmt"
	"time"
)

// Source identifies where a location update originated.
type Source string

const (
	SourceWhatsApp Source = "whatsapp"
	SourceBrowser  Source = "browser"
)

// LocationUpdate is the canonical location representation shared by
// WhatsApp and browser geolocation sources.
//
// Optional values are pointers because an absent value must remain absent;
// the converter must never invent accuracy, altitude, heading or speed.
type LocationUpdate struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Accuracy  *float64 `json:"accuracy"`
	Altitude  *float64 `json:"altitude"`
	Heading   *float64 `json:"heading"`
	Speed     *float64 `json:"speed"`
	Timestamp int64    `json:"timestamp"`
	Source    Source   `json:"source"`
}

// WhatsAppLocation is the normalized subset of the Evolution-Go webhook
// location object consumed by this converter.
type WhatsAppLocation struct {
	Latitude  float64
	Longitude float64
	Accuracy  *float64
	Altitude  *float64

	// SpeedMPS is the WhatsApp live-location speed in metres/second.
	SpeedMPS *float64

	// HeadingMagneticDegrees is WhatsApp's direction measured clockwise
	// from magnetic north. It is intentionally not mapped to browser
	// GeolocationCoordinates.heading because browser heading is based on
	// true north and magnetic declination is not available here.
	HeadingMagneticDegrees *float64

	// Timestamp accepts RFC3339/RFC3339Nano because Evolution-Go sends the
	// webhook timestamp in that format.
	Timestamp string
}

// BrowserGeolocationCoordinates is the JSON-compatible representation of
// the useful fields exposed by browser GeolocationCoordinates.
type BrowserGeolocationCoordinates struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Accuracy  float64  `json:"accuracy"`
	Altitude  *float64 `json:"altitude"`
	Heading   *float64 `json:"heading"`
	Speed     *float64 `json:"speed"`
}

// BrowserGeolocationPosition is the JSON-compatible representation of a
// browser GeolocationPosition. Timestamp is Unix milliseconds, matching
// GeolocationPosition.timestamp.
type BrowserGeolocationPosition struct {
	Coords    BrowserGeolocationCoordinates `json:"coords"`
	Timestamp int64                         `json:"timestamp"`
}

// FromWhatsApp converts an Evolution-Go/WhatsApp location into the
// canonical representation.
//
// WhatsApp live-location speed is preserved in m/s. Magnetic heading is
// deliberately left unset because it cannot safely be converted to the
// browser's true-north heading without magnetic declination data.
func FromWhatsApp(input WhatsAppLocation) (LocationUpdate, error) {
	if err := validateCoordinates(input.Latitude, input.Longitude); err != nil {
		return LocationUpdate{}, err
	}

	timestamp, err := parseTimestampMillis(input.Timestamp)
	if err != nil {
		return LocationUpdate{}, err
	}

	return LocationUpdate{
		Latitude:  input.Latitude,
		Longitude: input.Longitude,
		Accuracy:  input.Accuracy,
		Altitude:  input.Altitude,
		Heading:   nil,
		Speed:     input.SpeedMPS,
		Timestamp: timestamp,
		Source:    SourceWhatsApp,
	}, nil
}

// FromBrowser converts a browser GeolocationPosition-compatible value into
// the canonical representation.
func FromBrowser(input BrowserGeolocationPosition) (LocationUpdate, error) {
	if err := validateCoordinates(input.Coords.Latitude, input.Coords.Longitude); err != nil {
		return LocationUpdate{}, err
	}
	if input.Coords.Accuracy < 0 {
		return LocationUpdate{}, fmt.Errorf("accuracy must be >= 0")
	}
	if input.Timestamp < 0 {
		return LocationUpdate{}, fmt.Errorf("timestamp must be >= 0")
	}

	return LocationUpdate{
		Latitude:  input.Coords.Latitude,
		Longitude: input.Coords.Longitude,
		Accuracy:  &input.Coords.Accuracy,
		Altitude:  input.Coords.Altitude,
		Heading:   input.Coords.Heading,
		Speed:     input.Coords.Speed,
		Timestamp: input.Timestamp,
		Source:    SourceBrowser,
	}, nil
}

// ToBrowser converts the canonical representation into a browser-compatible
// JSON shape. Missing optional values remain null.
func ToBrowser(input LocationUpdate) (BrowserGeolocationPosition, error) {
	if err := validateCoordinates(input.Latitude, input.Longitude); err != nil {
		return BrowserGeolocationPosition{}, err
	}
	if input.Timestamp < 0 {
		return BrowserGeolocationPosition{}, fmt.Errorf("timestamp must be >= 0")
	}
	if input.Accuracy == nil {
		// GeolocationCoordinates.accuracy is required by the browser API.
		// A missing source value therefore becomes 0 only at this explicit
		// compatibility boundary; the canonical representation remains nil.
		return BrowserGeolocationPosition{
			Coords: BrowserGeolocationCoordinates{
				Latitude:  input.Latitude,
				Longitude: input.Longitude,
				Accuracy: 0,
				Altitude:  input.Altitude,
				Heading:   input.Heading,
				Speed:     input.Speed,
			},
			Timestamp: input.Timestamp,
		}, nil
	}

	return BrowserGeolocationPosition{
		Coords: BrowserGeolocationCoordinates{
			Latitude:  input.Latitude,
			Longitude: input.Longitude,
			Accuracy:  *input.Accuracy,
			Altitude:  input.Altitude,
			Heading:   input.Heading,
			Speed:     input.Speed,
		},
		Timestamp: input.Timestamp,
	}, nil
}

func validateCoordinates(latitude, longitude float64) error {
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude out of range: %v", latitude)
	}
	if longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude out of range: %v", longitude)
	}
	return nil
}

func parseTimestampMillis(value string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("timestamp is required")
	}

	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp %q: %w", value, err)
	}

	return timestamp.UnixMilli(), nil
}
