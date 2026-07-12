package enginewire

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"unicode/utf8"
)

// WireValue is one lossless projected/diff JSON value. Numbers remain
// json.Number lexemes and mutable slices/maps are copied on ingress and egress.
type WireValue struct {
	value any
}

// NewWireRecord recursively copies and normalizes one verified record map.
func NewWireRecord(values map[string]any) (WireRecord, error) {
	if values == nil {
		return nil, fmt.Errorf("%w: record must be an object", ErrInvalidFrame)
	}
	out := make(WireRecord, len(values))
	for key, value := range values {
		if err := validateDynamicKey(key); err != nil {
			return nil, err
		}
		converted, err := NewWireValue(value)
		if err != nil {
			return nil, err
		}
		out[key] = converted
	}
	return out, nil
}

// NewWireValue recursively copies one admitted JSON-compatible value.
func NewWireValue(value any) (WireValue, error) {
	cloned, err := cloneWireValue(value)
	if err != nil {
		return WireValue{}, err
	}
	return WireValue{value: cloned}, nil
}

// Value returns a recursively independent copy of the dynamic value.
func (v WireValue) Value() (any, error) {
	return cloneWireValue(v.value)
}

func (v WireValue) MarshalJSON() ([]byte, error) {
	if _, err := cloneWireValue(v.value); err != nil {
		return nil, err
	}
	return json.Marshal(v.value)
}

func (v *WireValue) UnmarshalJSON(data []byte) error {
	if v == nil {
		return fmt.Errorf("%w: nil wire-value destination", ErrInvalidFrame)
	}
	if err := validateJSONValue(data, V1JSONDepth); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%w: dynamic value decode: %v", ErrInvalidFrame, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%w: dynamic value trailing data", ErrInvalidFrame)
	}
	cloned, err := cloneWireValue(value)
	if err != nil {
		return err
	}
	v.value = cloned
	return nil
}

func cloneWireValue(value any) (any, error) {
	return cloneWireReflect(reflect.ValueOf(value), map[wireVisit]bool{}, 1)
}

type wireVisit struct {
	typeOf  reflect.Type
	pointer uintptr
}

func cloneWireReflect(value reflect.Value, active map[wireVisit]bool, depth int) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		return cloneWireReflect(value.Elem(), active, depth)
	}
	if value.CanInterface() {
		switch typed := value.Interface().(type) {
		case json.Number:
			if !validJSONNumberLexeme(typed.String()) {
				return nil, fmt.Errorf("%w: invalid dynamic number", ErrInvalidFrame)
			}
			return json.Number(typed.String()), nil
		case WireValue:
			return cloneWireValue(typed.value)
		case []byte:
			if typed == nil {
				return nil, nil
			}
			return base64.StdEncoding.EncodeToString(typed), nil
		}
	}
	if value.Type().PkgPath() != "" {
		return nil, fmt.Errorf("%w: unsupported named dynamic value type %s", ErrInvalidFrame, value.Type())
	}
	switch value.Kind() {
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.String:
		text := value.String()
		if !utf8.ValidString(text) {
			return nil, fmt.Errorf("%w: dynamic string is not valid UTF-8", ErrInvalidFrame)
		}
		return text, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return json.Number(strconv.FormatInt(value.Int(), 10)), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return json.Number(strconv.FormatUint(value.Uint(), 10)), nil
	case reflect.Float32, reflect.Float64:
		floating := value.Float()
		if math.IsNaN(floating) || math.IsInf(floating, 0) {
			return nil, fmt.Errorf("%w: dynamic float is not finite", ErrInvalidFrame)
		}
		return json.Number(strconv.FormatFloat(floating, 'g', -1, value.Type().Bits())), nil
	case reflect.Map:
		if depth > V1JSONDepth {
			return nil, fmt.Errorf("%w: dynamic value exceeds maximum JSON depth", ErrJSONDepth)
		}
		if value.Type() != reflect.TypeFor[map[string]any]() {
			return nil, fmt.Errorf("%w: dynamic map must be map[string]any", ErrInvalidFrame)
		}
		if value.IsNil() {
			return nil, nil
		}
		visit := wireVisit{typeOf: value.Type(), pointer: value.Pointer()}
		if active[visit] {
			return nil, fmt.Errorf("%w: cyclic dynamic map", ErrInvalidFrame)
		}
		active[visit] = true
		defer delete(active, visit)
		out := make(map[string]any, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			key := iterator.Key().String()
			if !utf8.ValidString(key) {
				return nil, fmt.Errorf("%w: dynamic object key is not valid UTF-8", ErrInvalidFrame)
			}
			cloned, err := cloneWireReflect(iterator.Value(), active, depth+1)
			if err != nil {
				return nil, err
			}
			out[key] = cloned
		}
		return out, nil
	case reflect.Slice:
		if depth > V1JSONDepth {
			return nil, fmt.Errorf("%w: dynamic value exceeds maximum JSON depth", ErrJSONDepth)
		}
		if value.IsNil() {
			return nil, nil
		}
		if !wireSequenceElementAllowed(value.Type().Elem()) {
			return nil, fmt.Errorf("%w: unsupported dynamic slice type %s", ErrInvalidFrame, value.Type())
		}
		visit := wireVisit{typeOf: value.Type(), pointer: value.Pointer()}
		if active[visit] {
			return nil, fmt.Errorf("%w: cyclic dynamic slice", ErrInvalidFrame)
		}
		active[visit] = true
		defer delete(active, visit)
		return cloneWireSequence(value, active, depth)
	case reflect.Array:
		if depth > V1JSONDepth {
			return nil, fmt.Errorf("%w: dynamic value exceeds maximum JSON depth", ErrJSONDepth)
		}
		if !wireSequenceElementAllowed(value.Type().Elem()) {
			return nil, fmt.Errorf("%w: unsupported dynamic array type %s", ErrInvalidFrame, value.Type())
		}
		return cloneWireSequence(value, active, depth)
	default:
		return nil, fmt.Errorf("%w: unsupported dynamic value type %s", ErrInvalidFrame, value.Type())
	}
}

func wireSequenceElementAllowed(element reflect.Type) bool {
	if element == reflect.TypeFor[any]() || element == reflect.TypeFor[map[string]any]() || element == reflect.TypeFor[json.Number]() {
		return true
	}
	if element.PkgPath() != "" {
		return false
	}
	switch element.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func cloneWireSequence(value reflect.Value, active map[wireVisit]bool, depth int) ([]any, error) {
	out := make([]any, value.Len())
	for index := range value.Len() {
		cloned, err := cloneWireReflect(value.Index(index), active, depth+1)
		if err != nil {
			return nil, err
		}
		out[index] = cloned
	}
	return out, nil
}

func validJSONNumberLexeme(value string) bool {
	data := []byte(`{"value":` + value + `}`)
	return validateJSONObject(data, V1JSONDepth) == nil
}
