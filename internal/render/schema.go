// Package render validates notification data and renders immutable channel payloads.
package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	spb "github.com/gopherex/schemapb/go/schemapb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/gopherex/courier/pkg/api"
)

// ErrSchema reports a schema outside Courier's strict validation policy.
var (
	ErrSchema = errors.New("invalid_schema")
	// ErrData reports data that does not conform to the channel schema.
	ErrData = errors.New("invalid_data")
)

// Compile rejects transformation features throughout nested schemas and definitions.
func Compile(input api.Schema) (*spb.Engine, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode schema: %w", err)
	}

	schema := &spb.Schema{}
	if err = protojson.Unmarshal(raw, schema); err != nil {
		return nil, ErrSchema
	}

	if policyErr := schemaPolicy(schema.ProtoReflect()); policyErr != nil {
		return nil, policyErr
	}

	engine, err := spb.Compile(schema, spb.WithCostLimit(validationCostLimit))
	if err != nil {
		return nil, ErrSchema
	}

	return engine, nil
}

func schemaPolicy(message protoreflect.Message) error {
	if schema, ok := message.Interface().(*spb.Schema); ok && (!schema.GetStrict() || schema.GetCoerce()) {
		return ErrSchema
	}

	var failure error

	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch field.Name() {
		case "computed", "normalize", "default", "when", "immutable":
			failure = ErrSchema
			return false
		default:
		}

		switch {
		case field.IsMap() && field.MapValue().Kind() == protoreflect.MessageKind:
			value.Map().Range(func(_ protoreflect.MapKey, item protoreflect.Value) bool {
				failure = schemaPolicy(item.Message())
				return failure == nil
			})
		case field.IsList() && field.Kind() == protoreflect.MessageKind:
			for index := range value.List().Len() {
				if failure = schemaPolicy(value.List().Get(index).Message()); failure != nil {
					break
				}
			}
		case !field.IsMap() && field.Kind() == protoreflect.MessageKind:
			failure = schemaPolicy(value.Message())
		}

		return failure == nil
	})

	return failure
}

// Values preserves integers while decoding dynamic JSON without string coercion.
func Values(data api.Data) (map[string]any, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode data: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var result map[string]any
	if err = decoder.Decode(&result); err != nil {
		return nil, ErrData
	}

	for key, value := range result {
		result[key], err = numbers(value)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func numbers(value any) (any, error) {
	switch item := value.(type) {
	case json.Number:
		if strings.ContainsAny(string(item), ".eE") {
			number, err := strconv.ParseFloat(string(item), 64)
			if err != nil {
				return nil, ErrData
			}

			return number, nil
		}

		if number, err := strconv.ParseInt(string(item), 10, 64); err == nil {
			return number, nil
		}

		number, err := strconv.ParseUint(string(item), 10, 64)
		if err != nil {
			return nil, ErrData
		}

		return number, nil
	case map[string]any:
		for key, child := range item {
			converted, err := numbers(child)
			if err != nil {
				return nil, err
			}

			item[key] = converted
		}
	case []any:
		for index, child := range item {
			converted, err := numbers(child)
			if err != nil {
				return nil, err
			}

			item[index] = converted
		}
	default:
	}

	return value, nil
}

const validationCostLimit = 10000
