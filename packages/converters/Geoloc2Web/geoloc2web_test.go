package geoloc2web

import (
	"encoding/json"
	"testing"
)

func float64ptr(value float64) *float64 {
	return &value
}

func TestFromWhatsApp(t *testing.T) {
	input := WhatsAppLocation{
		Latitude:              -24.1234024,
		Longitude:             -49.3515181,
		Accuracy:              float64ptr(8.4),
		Altitude:              float64ptr(823.2),
		SpeedMPS:              float64ptr(1.8),
		HeadingMagneticDegrees: float64ptr(143.5),
		Timestamp:             "2026-10-05T05:40:17.123Z",
	}

	got, err := FromWhatsApp(input)
	if err != nil {
		t.Fatalf("FromWhatsApp() error = %v", err)
	}

	if got.Latitude != input.Latitude || got.Longitude != input.Longitude {
		t.Fatalf("coordinates = (%v, %v), want (%v, %v)", got.Latitude, got.Longitude, input.Latitude, input.Longitude)
	}
	if got.Accuracy == nil || *got.Accuracy != 8.4 {
		t.Fatalf("accuracy = %v, want 8.4", got.Accuracy)
	}
	if got.Altitude == nil || *got.Altitude != 823.2 {
		t.Fatalf("altitude = %v, want 823.2", got.Altitude)
	}
	if got.Speed == nil || *got.Speed != 1.8 {
		t.Fatalf("speed = %v, want 1.8", got.Speed)
	}
	if got.Heading != nil {
		t.Fatalf("heading = %v, want nil because WhatsApp heading is magnetic", got.Heading)
	}
	if got.Timestamp != 1791178817123 {
		t.Fatalf("timestamp = %v, want 1791178817123", got.Timestamp)
	}
	if got.Source != SourceWhatsApp {
		t.Fatalf("source = %q, want %q", got.Source, SourceWhatsApp)
	}
}

func TestFromWhatsAppPreservesMissingOptionalFields(t *testing.T) {
	got, err := FromWhatsApp(WhatsAppLocation{
		Latitude:  -24.123,
		Longitude: -49.351,
		Timestamp: "2026-10-05T05:40:17Z",
	})
	if err != nil {
		t.Fatalf("FromWhatsApp() error = %v", err)
	}

	if got.Accuracy != nil || got.Altitude != nil || got.Heading != nil || got.Speed != nil {
		t.Fatalf("optional fields should remain nil: %+v", got)
	}
}

func TestFromBrowser(t *testing.T) {
	input := BrowserGeolocationPosition{
		Coords: BrowserGeolocationCoordinates{
			Latitude:  -24.1234024,
			Longitude: -49.3515181,
			Accuracy:  8.4,
			Altitude:  float64ptr(823.2),
			Heading:   float64ptr(143.5),
			Speed:     float64ptr(1.8),
		},
		Timestamp: 1791178817123,
	}

	got, err := FromBrowser(input)
	if err != nil {
		t.Fatalf("FromBrowser() error = %v", err)
	}

	if got.Source != SourceBrowser {
		t.Fatalf("source = %q, want %q", got.Source, SourceBrowser)
	}
	if got.Timestamp != input.Timestamp {
		t.Fatalf("timestamp = %v, want %v", got.Timestamp, input.Timestamp)
	}
	if got.Heading == nil || *got.Heading != 143.5 {
		t.Fatalf("heading = %v, want 143.5", got.Heading)
	}
}

func TestToBrowser(t *testing.T) {
	input := LocationUpdate{
		Latitude:  -24.1234024,
		Longitude: -49.3515181,
		Accuracy:  float64ptr(8.4),
		Altitude:  float64ptr(823.2),
		Heading:   float64ptr(143.5),
		Speed:     float64ptr(1.8),
		Timestamp: 1791178817123,
		Source:    SourceWhatsApp,
	}

	got, err := ToBrowser(input)
	if err != nil {
		t.Fatalf("ToBrowser() error = %v", err)
	}

	if got.Coords.Latitude != input.Latitude || got.Coords.Longitude != input.Longitude {
		t.Fatalf("coordinates = (%v, %v), want (%v, %v)", got.Coords.Latitude, got.Coords.Longitude, input.Latitude, input.Longitude)
	}
	if got.Coords.Accuracy != 8.4 {
		t.Fatalf("accuracy = %v, want 8.4", got.Coords.Accuracy)
	}
	if got.Timestamp != input.Timestamp {
		t.Fatalf("timestamp = %v, want %v", got.Timestamp, input.Timestamp)
	}
}

func TestToBrowserMissingAccuracyUsesCompatibilityZero(t *testing.T) {
	input := LocationUpdate{
		Latitude:  -24.123,
		Longitude: -49.351,
		Timestamp: 1791178817000,
		Source:    SourceWhatsApp,
	}

	got, err := ToBrowser(input)
	if err != nil {
		t.Fatalf("ToBrowser() error = %v", err)
	}
	if got.Coords.Accuracy != 0 {
		t.Fatalf("accuracy = %v, want 0 at browser compatibility boundary", got.Coords.Accuracy)
	}
}

func TestInvalidWhatsAppCoordinates(t *testing.T) {
	_, err := FromWhatsApp(WhatsAppLocation{
		Latitude:  91,
		Longitude: -49,
		Timestamp: "2026-10-05T05:40:17Z",
	})
	if err == nil {
		t.Fatal("FromWhatsApp() expected latitude validation error")
	}
}

func TestInvalidWhatsAppTimestamp(t *testing.T) {
	_, err := FromWhatsApp(WhatsAppLocation{
		Latitude:  -24,
		Longitude: -49,
		Timestamp: "not-a-timestamp",
	})
	if err == nil {
		t.Fatal("FromWhatsApp() expected timestamp validation error")
	}
}

func TestBrowserJSONShape(t *testing.T) {
	position := BrowserGeolocationPosition{
		Coords: BrowserGeolocationCoordinates{
			Latitude:  -24.1234024,
			Longitude: -49.3515181,
			Accuracy:  8.4,
			Altitude:  nil,
			Heading:   nil,
			Speed:     nil,
		},
		Timestamp: 1791178817123,
	}

	data, err := json.Marshal(position)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	want := `{"coords":{"latitude":-24.1234024,"longitude":-49.3515181,"accuracy":8.4,"altitude":null,"heading":null,"speed":null},"timestamp":1791178817123}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}
