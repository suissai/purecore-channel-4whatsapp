package whatsmeow_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type geolocationWebhookPayload struct {
	Event        string                 `json:"event"`
	MessageID    string                 `json:"messageId"`
	ChatJID      string                 `json:"chatJid"`
	SenderJID    string                 `json:"senderJid"`
	Timestamp    string                 `json:"timestamp"`
	Location     map[string]interface{} `json:"location"`
	LiveLocation map[string]interface{} `json:"liveLocation,omitempty"`
}

func protoField(msg protoreflect.Message, names ...string) (protoreflect.Value, protoreflect.FieldDescriptor, bool) {
	for _, name := range names {
		fd := msg.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd != nil {
			return msg.Get(fd), fd, true
		}
	}
	return protoreflect.Value{}, nil, false
}

func protoString(msg protoreflect.Message, names ...string) (string, bool) {
	v, fd, ok := protoField(msg, names...)
	if !ok || fd.Kind() != protoreflect.StringKind {
		return "", false
	}
	return v.String(), true
}

func protoBool(msg protoreflect.Message, names ...string) bool {
	v, fd, ok := protoField(msg, names...)
	return ok && fd.Kind() == protoreflect.BoolKind && v.Bool()
}

func protoNumber(msg protoreflect.Message, names ...string) (float64, bool) {
	v, fd, exists := protoField(msg, names...)
	if !exists {
		return 0, false
	}

	switch fd.Kind() {
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return v.Float(), true
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return float64(v.Int()), true
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return float64(v.Uint()), true
	default:
		return 0, false
	}
}

func extractGeolocation(evt *events.Message) (*geolocationWebhookPayload, bool) {
	if evt == nil || evt.Message == nil {
		return nil, false
	}

	location := evt.Message.GetLocationMessage()
	if location == nil {
		return nil, false
	}

	msg := location.ProtoReflect()

	latitude, okLat := protoNumber(msg, "degrees_latitude", "degreesLatitude")
	longitude, okLon := protoNumber(msg, "degrees_longitude", "degreesLongitude")
	if !okLat || !okLon {
		return nil, false
	}

	isLive := protoBool(msg, "is_live", "isLive")
	eventName := "message.location"
	if isLive {
		eventName = "message.live_location"
	}

	locationData := map[string]interface{}{
		"latitude":  latitude,
		"longitude": longitude,
	}

	if value, ok := protoNumber(msg, "accuracy_in_meters", "accuracyInMeters"); ok {
		locationData["accuracy"] = value
	}
	if value, ok := protoNumber(msg, "altitude"); ok {
		locationData["altitude"] = value
	}
	if value, ok := protoString(msg, "name"); ok && value != "" {
		locationData["name"] = value
	}
	if value, ok := protoString(msg, "address"); ok && value != "" {
		locationData["address"] = value
	}
	if value, ok := protoString(msg, "url"); ok && value != "" {
		locationData["url"] = value
	}

	payload := &geolocationWebhookPayload{
		Event:     eventName,
		MessageID: evt.Info.ID,
		ChatJID:   evt.Info.Chat.String(),
		SenderJID: evt.Info.Sender.String(),
		Timestamp: evt.Info.Timestamp.Format(time.RFC3339Nano),
		Location:  locationData,
	}

	if isLive {
		live := map[string]interface{}{}
		if value, ok := protoNumber(msg, "accuracy_in_meters", "accuracyInMeters"); ok {
			live["accuracyInMeters"] = value
		}
		if value, ok := protoNumber(msg, "speed_in_mps", "speedInMps"); ok {
			live["speedInMps"] = value
		}
		if value, ok := protoNumber(msg, "degrees_clockwise_from_magnetic_north", "degreesClockwiseFromMagneticNorth"); ok {
			live["degreesClockwiseFromMagneticNorth"] = value
		}
		if value, ok := protoNumber(msg, "sequence_number", "sequenceNumber"); ok {
			live["sequenceNumber"] = value
		}
		if len(live) > 0 {
			payload.LiveLocation = live
		}
	}

	return payload, true
}

func (mycli *MyClient) dispatchGeolocationWebhook(evt *events.Message, instanceID string) {
	if mycli == nil || mycli.config == nil || mycli.config.GeolocationWebhookURL == "" {
		return
	}

	payload, ok := extractGeolocation(evt)
	if !ok {
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		mycli.loggerWrapper.GetLogger(instanceID).LogError("[%s] Failed to marshal geolocation webhook payload: %v", instanceID, err)
		return
	}

	go func() {
		const attempts = 3
		client := &http.Client{Timeout: 10 * time.Second}

		for attempt := 1; attempt <= attempts; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, mycli.config.GeolocationWebhookURL, bytes.NewReader(body))
			if reqErr == nil {
				req.Header.Set("Content-Type", "application/json")
				if mycli.config.GeolocationWebhookSecret != "" {
					req.Header.Set("X-Webhook-Secret", w.config.GeolocationWebhookSecret)
				}

				resp, doErr := client.Do(req)
				if doErr == nil {
					resp.Body.Close()
					cancel()

					if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
						w.loggerWrapper.GetLogger(instanceID).LogInfo(
							"[%s] Geolocation webhook delivered: event=%s messageId=%s status=%d",
							instanceID, payload.Event, payload.MessageID, resp.StatusCode,
						)
						return
					}

					reqErr = fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
				} else {
					reqErr = doErr
				}
			}
			cancel()

			if attempt < attempts {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			} else {
				w.loggerWrapper.GetLogger(instanceID).LogError(
					"[%s] Failed to deliver geolocation webhook after %d attempts: %v",
					instanceID, attempts, reqErr,
				)
			}
		}
	}()
}
