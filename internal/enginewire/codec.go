package enginewire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

var wireValueType = reflect.TypeFor[WireValue]()

type frameValidator interface {
	validate() error
}

func decodeExact(data []byte, destination any) error {
	typ := reflect.TypeOf(destination)
	if typ == nil || typ.Kind() != reflect.Pointer || reflect.ValueOf(destination).IsNil() {
		return fmt.Errorf("%w: decode destination must be a nonnil pointer", ErrInvalidFrame)
	}
	if err := checkRequiredJSON(data, typ.Elem()); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: exact DTO decode: %v", ErrInvalidFrame, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: trailing JSON value", ErrInvalidFrame)
		}
		return fmt.Errorf("%w: trailing JSON decode: %v", ErrInvalidFrame, err)
	}
	return nil
}

func decodeFrame[T frameValidator](data []byte) (T, error) {
	var frame T
	if err := decodeExact(data, &frame); err != nil {
		return frame, err
	}
	if err := frame.validate(); err != nil {
		return frame, err
	}
	return frame, nil
}

func checkRequiredJSON(data []byte, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == wireValueType {
		return nil
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("%w: null is not valid for %s", ErrInvalidFrame, typ)
	}
	switch typ.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return fmt.Errorf("%w: required-field object decode: %v", ErrInvalidFrame, err)
		}
		allowed := make(map[string]struct{}, typ.NumField())
		for index := range typ.NumField() {
			field := typ.Field(index)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			if name == "-" {
				continue
			}
			allowed[name] = struct{}{}
			raw, present := object[name]
			if field.Tag.Get("wire") == "required" && !present {
				return fmt.Errorf("%w: required member %q is missing", ErrInvalidFrame, name)
			}
			if !present {
				continue
			}
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && !allowsJSONNull(field.Type) {
				return fmt.Errorf("%w: member %q cannot be null", ErrInvalidFrame, name)
			}
			if err := checkRequiredJSON(raw, field.Type); err != nil {
				return err
			}
		}
		for name := range object {
			if _, ok := allowed[name]; !ok {
				return fmt.Errorf("%w: unknown member %q", ErrInvalidFrame, name)
			}
		}
	case reflect.Slice, reflect.Array:
		var values []json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return fmt.Errorf("%w: required-field array decode: %v", ErrInvalidFrame, err)
		}
		for _, value := range values {
			if err := checkRequiredJSON(value, typ.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		if typ.Elem() == wireValueType {
			return nil
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return fmt.Errorf("%w: required-field map decode: %v", ErrInvalidFrame, err)
		}
		for _, value := range values {
			if err := checkRequiredJSON(value, typ.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func allowsJSONNull(typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ == wireValueType
}

func discriminator(data []byte, name string) (string, bool, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return "", false, fmt.Errorf("%w: discriminator object: %v", ErrInvalidFrame, err)
	}
	raw, ok := object[name]
	if !ok {
		return "", false, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", true, fmt.Errorf("%w: discriminator %q must be a string", ErrInvalidFrame, name)
	}
	return value, true, nil
}

func nestedDiscriminator(data []byte, objectName, name string) (string, bool, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return "", false, fmt.Errorf("%w: nested discriminator object: %v", ErrInvalidFrame, err)
	}
	nested, ok := object[objectName]
	if !ok {
		return "", false, nil
	}
	return discriminator(nested, name)
}

func marshalBounded(frame frameValidator, maximumBytes, maximumDepth int) ([]byte, error) {
	if isNilInterface(frame) {
		return nil, fmt.Errorf("%w: nil frame", ErrInvalidFrame)
	}
	if err := frame.validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("%w: frame encode: %v", ErrInvalidFrame, err)
	}
	if len(data) > maximumBytes {
		return nil, ErrFrameTooLarge
	}
	if err := validateJSONObject(data, maximumDepth); err != nil {
		return nil, fmt.Errorf("%w: encoded frame failed strict validation: %v", ErrInvalidFrame, err)
	}
	return data, nil
}

func writeBounded(writer io.Writer, frame frameValidator, maximumBytes, maximumDepth int) error {
	if isNilInterface(writer) {
		return fmt.Errorf("%w: nil frame writer", ErrInvalidFrame)
	}
	data, err := marshalBounded(frame, maximumBytes, maximumDepth)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	for len(data) > 0 {
		written, writeErr := writer.Write(data)
		if written < 0 || written > len(data) {
			return fmt.Errorf("%w: invalid writer byte count", ErrInvalidFrame)
		}
		data = data[written:]
		if writeErr != nil {
			return writeErr
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
