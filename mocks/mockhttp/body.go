package mockhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"reflect"
)

// bodyBytes converts the various forms a http request/response body might take to bytes.
func bodyBytes(body interface{}) ([]byte, error) {
	switch v := body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case io.Reader:
		panic("Readers should be read explicitly, since they aren't idempotent")
	default:
		return json.Marshal(v)
	}
}

// bodiesEqual checks if two http request/response body-like objects are equal.
func bodiesEqual(a, b interface{}) (bool, error) {
	// Combinations! Argh!
	switch av := a.(type) {
	case nil:
		switch bv := b.(type) {
		case nil:
			return true, nil
		case []byte:
			return (len(bv) == 0), nil
		case string:
			return (len(bv) == 0), nil
		case io.Reader:
			panic("Readers should be read explicitly, since they aren't idempotent")
		default:
			return false, nil
		}
	case []byte:
		switch bv := b.(type) {
		case nil:
			return (len(av) == 0), nil
		case []byte:
			return bytes.Equal(av, bv), nil
		case string:
			return bytes.Equal(av, []byte(bv)), nil
		case io.Reader:
			panic("Readers should be read explicitly, since they aren't idempotent")
		default:
			return jsonEqual(av, bv)
		}
	case string:
		switch bv := b.(type) {
		case nil:
			return (av == ""), nil
		case []byte:
			return bytes.Equal([]byte(av), bv), nil
		case string:
			return (av == bv), nil
		case io.Reader:
			data, err := ioutil.ReadAll(bv)
			if err != nil {
				return false, err
			}
			return bytes.Equal([]byte(av), data), nil
		default:
			return jsonEqual([]byte(av), bv)
		}
	case io.Reader:
		panic("Readers should be read explicitly, since they aren't idempotent")
	default:
		switch bv := b.(type) {
		case nil:
			return false, nil
		case []byte:
			return jsonEqual(bv, av)
		case string:
			return jsonEqual([]byte(bv), av)
		case io.Reader:
			panic("Readers should be read explicitly, since they aren't idempotent")
		default:
			data, err := json.Marshal(av)
			if err != nil {
				return false, err
			}
			return jsonEqual(data, bv)
		}
	}
}

// jsonEqual checks if some JSON data bytes are equal to a value v when unmarshaled.
//
// Only exported fields in v with a "json" tag are considered for equality.
// If data contains fields that aren't are exported/tagged in v, an error is returned.
func jsonEqual(data []byte, v interface{}) (bool, error) {
	// Step 1: unmarshal the data into something of the same type as v.
	t := reflect.TypeOf(v)
	n := reflect.New(t).Interface()
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields() // Error if data contains fields that aren't in type-of-v.
	if err := d.Decode(n); err != nil {
		return false, err
	}

	// Step 2: marshal, than unmarshal, v.
	// Type-of-v might contain non-json/non-exported fields that would fail a direct equality match.
	// By marshaling and unmarshaling v we can set all extraneous fields to their zero value.
	vd, err := json.Marshal(v)
	if err != nil {
		return false, err
	}
	v2 := reflect.New(t).Interface()
	if err := json.Unmarshal(vd, v2); err != nil {
		return false, err
	}

	return reflect.DeepEqual(n, v2), nil
}
