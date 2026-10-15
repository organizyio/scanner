package item

import (
	"encoding/json"
)

func exifKnownKey(k string) bool {
	switch k {
	case "camera", "lens", "gps_lat", "gps_lon":
		return true
	default:
		return false
	}
}

// UnmarshalJSON decodes the exif object, routing serde-flattened keys into Extra.
func (e *ExifInfo) UnmarshalJSON(data []byte) error {
	*e = ExifInfo{}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for k, v := range m {
		if !exifKnownKey(k) {
			if e.Extra == nil {
				e.Extra = make(map[string]json.RawMessage)
			}
			e.Extra[k] = v
			continue
		}
		if string(v) == "null" {
			continue
		}
		switch k {
		case "camera", "lens":
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			if k == "camera" {
				e.Camera = &s
			} else {
				e.Lens = &s
			}
		case "gps_lat", "gps_lon":
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				return err
			}
			if k == "gps_lat" {
				e.GpsLat = &f
			} else {
				e.GpsLon = &f
			}
		}
	}
	return nil
}

// MarshalJSON writes known fields plus Extra as a single flat object.
func (e ExifInfo) MarshalJSON() ([]byte, error) {
	m := make(map[string]json.RawMessage)
	if e.Camera != nil {
		b, err := json.Marshal(*e.Camera)
		if err != nil {
			return nil, err
		}
		m["camera"] = b
	}
	if e.Lens != nil {
		b, err := json.Marshal(*e.Lens)
		if err != nil {
			return nil, err
		}
		m["lens"] = b
	}
	if e.GpsLat != nil {
		b, err := json.Marshal(*e.GpsLat)
		if err != nil {
			return nil, err
		}
		m["gps_lat"] = b
	}
	if e.GpsLon != nil {
		b, err := json.Marshal(*e.GpsLon)
		if err != nil {
			return nil, err
		}
		m["gps_lon"] = b
	}
	for k, v := range e.Extra {
		m[k] = v
	}
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}
