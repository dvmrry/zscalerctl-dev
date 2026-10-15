package zscaler

import (
	"reflect"
	"strings"
	"sync"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// preserveEmptyLists wraps a record mapper so a top-level list the API
// returned as [] reaches the record as a confirmed-empty list instead of being
// dropped. The SDK decodes an absent key to a nil slice and [] to an empty
// non-nil one, so the distinction is still there in T; the mappers' own
// len(x) > 0 guards are what discard it.
//
// Only fields the mapper copies under their own JSON name are preserved. That
// is established once per type by mapping a probe value whose list holds one
// zero element and checking that the same key comes out; a renamed, derived
// or filtered field is never guessed at and keeps today's behavior.
func preserveEmptyLists[T any](mapper func(T) resources.SourceRecord) func(T) resources.SourceRecord {
	var (
		once   sync.Once
		fields []emptyListField
	)
	return func(item T) resources.SourceRecord {
		record := mapper(item)
		once.Do(func() { fields = identityListFields(mapper) })
		if len(fields) == 0 {
			return record
		}
		value := reflect.ValueOf(item)
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return record
			}
			value = value.Elem()
		}
		for _, field := range fields {
			list := value.Field(field.index)
			if !list.IsNil() && list.Len() == 0 {
				record = record.WithConfirmedEmptyList(field.name)
			}
		}
		return record
	}
}

type emptyListField struct {
	index int
	name  string
}

// identityListFields returns T's top-level slice fields that mapper copies
// under their JSON name: mapping a value whose list holds one zero element
// yields a record with that key. A mapper that panics on the probe value
// simply confirms nothing for that field.
func identityListFields[T any](mapper func(T) resources.SourceRecord) []emptyListField {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil || typ.Kind() != reflect.Struct {
		return nil
	}
	var out []emptyListField
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() || field.Type.Kind() != reflect.Slice {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		probe := reflect.New(typ).Elem()
		probe.Field(i).Set(reflect.MakeSlice(field.Type, 1, 1))
		if mapsUnderName(mapper, probe.Interface().(T), name) {
			out = append(out, emptyListField{index: i, name: name})
		}
	}
	return out
}

func mapsUnderName[T any](mapper func(T) resources.SourceRecord, probe T, name string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return mapper(probe).Has(name)
}
