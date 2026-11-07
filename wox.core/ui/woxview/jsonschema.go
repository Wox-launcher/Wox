package woxview

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
)

type schemaObject map[string]any
type enumOwner interface{ enumValues() []string }

// JSONSchema describes the canonical JSON spelling of the versioned component catalog.
// Cross-node rules (identity, form ownership, field names) and byte/depth budgets
// remain enforced by Parse; JSON Schema cannot express those recursive constraints.
func JSONSchema() []byte {
	definitions := schemaObject{}
	for _, t := range []reflect.Type{reflect.TypeFor[Style](), reflect.TypeFor[StateStyle](), reflect.TypeFor[Transition](), reflect.TypeFor[Animation](), reflect.TypeFor[Timeline](), reflect.TypeFor[PresenceAnimation](), reflect.TypeFor[Color](), reflect.TypeFor[Edges](), reflect.TypeFor[Size](), reflect.TypeFor[Offset](), reflect.TypeFor[Handler]()} {
		definitions[t.Name()] = schemaFor(t)
	}
	definitions["EventData"] = schemaFor(reflect.TypeFor[Event]())
	types := make([]NodeType, 0, len(catalog))
	for kind := range catalog {
		types = append(types, kind)
	}
	slices.Sort(types)
	variants := []any{}
	for _, kind := range types {
		spec := catalog[kind]
		props := schemaFor(reflect.TypeOf(spec.newProps()).Elem())
		if kind == NodeSubmit {
			delete(props["properties"].(schemaObject), "onPress")
		}
		definitions[string(kind)+"Props"] = props
		children := schemaObject{"type": "array", "items": schemaObject{"$ref": "#/$defs/node"}}
		if !spec.children {
			children["maxItems"] = 0
		}
		if kind == NodeText {
			children["items"] = schemaObject{"$ref": "#/$defs/spanNode"}
		}
		if kind == NodeLazyList {
			children["items"] = schemaObject{"allOf": []any{schemaObject{"$ref": "#/$defs/node"}, schemaObject{"required": []string{"key"}, "properties": schemaObject{"key": schemaObject{"type": "string", "minLength": 1}}}}}
		}
		node := schemaObject{
			"type": "object", "additionalProperties": false, "required": []string{"type"},
			"properties": schemaObject{
				"type":     schemaObject{"const": kind},
				"key":      schemaObject{"type": "string", "maxLength": 256},
				"props":    schemaObject{"anyOf": []any{schemaObject{"$ref": "#/$defs/" + string(kind) + "Props"}, schemaObject{"type": "null"}}},
				"children": children,
			},
		}
		if kind == NodeSprite || kind == NodeImage || kind == NodeLineChart || kind == NodeBarChart || kind == NodePieChart || kind == NodeScatterChart || kind == NodeMorphPath || kind == NodeIconButton || kind == NodeGrid || kind == NodeLazyList || kind == NodeTable || kind == NodeLink {
			node["required"] = []string{"type", "props"}
			node["properties"].(schemaObject)["props"] = schemaObject{"$ref": "#/$defs/" + string(kind) + "Props"}
		}
		if kind == NodeSpan {
			definitions["spanNode"] = node
		} else {
			variants = append(variants, node)
		}
	}
	definitions["node"] = schemaObject{"oneOf": variants}
	root := schemaObject{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title":   "WoxView version 1", "type": "object", "additionalProperties": false,
		"required":   []string{"version", "root"},
		"properties": schemaObject{"version": schemaObject{"const": SchemaVersion}, "root": schemaObject{"$ref": "#/$defs/node"}},
		"$defs":      definitions,
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		panic(err)
	} // All schema values above are JSON primitives.
	return append(data, '\n')
}

// schemaType references shared definitions to keep the generated contract compact.
func schemaType(t reflect.Type) schemaObject {
	switch t {
	case reflect.TypeFor[Style](), reflect.TypeFor[StateStyle](), reflect.TypeFor[Transition](), reflect.TypeFor[Animation](), reflect.TypeFor[Timeline](), reflect.TypeFor[PresenceAnimation](), reflect.TypeFor[Color](), reflect.TypeFor[Edges](), reflect.TypeFor[Size](), reflect.TypeFor[Offset](), reflect.TypeFor[Handler]():
		return schemaObject{"$ref": "#/$defs/" + t.Name()}
	default:
		return schemaFor(t)
	}
}

// schemaFor flattens embedded props and honors JSON tags, including private derived data.
func schemaFor(t reflect.Type) schemaObject {
	if t.Kind() == reflect.Pointer {
		return schemaObject{"anyOf": []any{schemaType(t.Elem()), schemaObject{"type": "null"}}}
	}
	if e, ok := reflect.Zero(t).Interface().(enumOwner); ok {
		return schemaObject{"type": "string", "enum": e.enumValues()}
	}
	switch t {
	case reflect.TypeFor[PresenceAnimation]():
		preset := schemaFor(reflect.TypeFor[PresencePreset]())
		return schemaObject{"anyOf": []any{preset, schemaObject{
			"type": "object", "additionalProperties": false, "required": []string{"type"},
			"properties": schemaObject{"type": preset, "duration": schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 0, "maximum": 60000}, schemaObject{"type": "null"}}}},
		}}}
	case reflect.TypeFor[Offset]():
		return schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": -16384, "maximum": 16384}, schemaObject{"type": "string", "enum": SizeToken("").enumValues()}, schemaObject{"type": "null"}}}
	case reflect.TypeFor[Size]():
		return schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": 0, "maximum": 16384}, schemaObject{"type": "string", "enum": SizeToken("").enumValues()}, schemaObject{"type": "null"}}}
	case reflect.TypeFor[Color]():
		value := schemaObject{"type": "string", "pattern": `^(|#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|#[0-9a-fA-F]{8}|\$(text|textSecondary|accent|accentText|background|surface|border|success|warning|error|hover|selected|selectedText|focus|disabled|link|overlay|warningText))$`}
		branch := schemaObject{"allOf": []any{value, schemaObject{"minLength": 1}}}
		return schemaObject{"anyOf": []any{value, schemaObject{"type": "null"}, schemaObject{
			"type": "object", "additionalProperties": false, "required": []string{"light", "dark"},
			"properties": schemaObject{"light": branch, "dark": branch},
		}}}
	case reflect.TypeFor[Handler]():
		return schemaObject{"type": "object", "additionalProperties": false, "required": []string{"$handler"}, "properties": schemaObject{"$handler": schemaObject{"type": "string", "minLength": 1, "maxLength": 128}}}
	case reflect.TypeFor[Edges]():
		props := schemaObject{}
		for _, side := range []string{"top", "right", "bottom", "left", "horizontal", "vertical"} {
			props[side] = schemaType(reflect.TypeFor[Size]())
		}
		return schemaObject{"anyOf": []any{schemaType(reflect.TypeFor[Size]()), schemaObject{"type": "object", "additionalProperties": false, "properties": props}}}
	}
	switch t.Kind() {
	case reflect.String:
		return schemaObject{"type": "string"}
	case reflect.Bool:
		return schemaObject{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return schemaObject{"type": "integer"}
	case reflect.Uint64:
		return schemaObject{"type": "integer", "minimum": 0, "maximum": uint64(1<<64 - 1)}
	case reflect.Float32, reflect.Float64:
		return schemaObject{"type": "number"}
	case reflect.Slice:
		return schemaObject{"type": "array", "items": schemaType(t.Elem())}
	case reflect.Struct:
		props := schemaObject{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			if field.Anonymous {
				for name, s := range schemaFor(field.Type)["properties"].(schemaObject) {
					props[name] = s
				}
				continue
			}
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			if t == reflect.TypeFor[Event]() && field.Name == "Args" {
				// Event arguments are encoded as a JSON array, not a byte array.
				props[name] = schemaObject{"type": "array", "items": schemaObject{}}
				continue
			}
			props[name] = schemaType(field.Type)
		}
		result := schemaObject{"type": "object", "additionalProperties": false, "properties": props}
		if t == reflect.TypeFor[Event]() {
			result["required"] = []string{"Handler", "Args", "Key"}
		}
		if t == reflect.TypeFor[Animation]() {
			result["required"] = []string{"type"}
			props["duration"] = schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 1, "maximum": 60000}, schemaObject{"type": "null"}}}
		}
		if t == reflect.TypeFor[SpriteProps]() {
			result["required"] = []string{"source", "columns", "rows"}
			props["source"] = schemaType(reflect.TypeFor[ImageSource]())
			for _, name := range []string{"columns", "rows"} {
				props[name] = schemaObject{"type": "integer", "minimum": 1, "maximum": 64}
			}
			props["frame"] = schemaObject{"type": "integer", "minimum": 0, "maximum": 4095}
			props["clips"].(schemaObject)["maxItems"] = 64
		}
		if t == reflect.TypeFor[SpriteClip]() {
			result["required"] = []string{"name", "frames", "fps"}
			props["name"] = schemaObject{"type": "string", "minLength": 1, "maxLength": 128}
			props["fps"] = schemaObject{"type": "number", "minimum": 1, "maximum": 60}
			props["frames"] = schemaObject{"type": "array", "minItems": 1, "maxItems": 256, "items": schemaObject{"type": "integer", "minimum": 0, "maximum": 4095}}
		}
		if t == reflect.TypeFor[Timeline]() {
			result["required"] = []string{"duration", "keyframes"}
			props["duration"] = schemaObject{"type": "integer", "minimum": 1, "maximum": 600000}
			props["keyframes"].(schemaObject)["minItems"] = 2
			props["keyframes"].(schemaObject)["maxItems"] = 256
		}
		if t == reflect.TypeFor[TimelineKeyframe]() {
			result["required"] = []string{"at"}
			props["at"] = schemaObject{"type": "integer", "minimum": 0, "maximum": 600000}
			for _, name := range []string{"translateX", "translateY", "rotate", "scale", "opacity"} {
				low, high := -16384, 16384
				if name == "scale" {
					low, high = 0, 100
				}
				if name == "opacity" {
					low, high = 0, 1
				}
				props[name] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": low, "maximum": high}, schemaObject{"type": "null"}}}
			}
		}
		if t == reflect.TypeFor[SpriteProps]() || t == reflect.TypeFor[Timeline]() || t == reflect.TypeFor[TimerProps]() {
			props["elapsed"] = schemaObject{"type": "integer", "minimum": 0, "maximum": 86400000}
			props["playbackId"] = schemaObject{"type": "string", "maxLength": 256}
		}
		if t == reflect.TypeFor[TimerProps]() {
			props["elapsed"] = schemaObject{"type": "integer", "minimum": 0, "maximum": maxTimerMilliseconds}
			props["duration"] = schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 0, "maximum": maxTimerMilliseconds}, schemaObject{"type": "null"}}}
			result["if"] = schemaObject{"anyOf": []any{
				schemaObject{"required": []string{"direction"}, "properties": schemaObject{"direction": schemaObject{"const": TimerDown}}},
				schemaObject{"required": []string{"onComplete"}, "properties": schemaObject{"onComplete": schemaObject{"type": "object"}}},
			}}
			result["then"] = schemaObject{"required": []string{"duration"}, "properties": schemaObject{"duration": schemaObject{"type": "integer"}}}
		}
		if t == reflect.TypeFor[LineChartProps]() || t == reflect.TypeFor[BarChartProps]() || t == reflect.TypeFor[PieChartProps]() {
			result["required"] = []string{"values"}
			limit := 4096
			minValue := -1e100
			if t == reflect.TypeFor[PieChartProps]() {
				limit, minValue = 128, 0
			}
			props["values"].(schemaObject)["maxItems"] = limit
			props["pointLabels"].(schemaObject)["maxItems"] = limit
			props["pointLabels"].(schemaObject)["items"] = schemaObject{"type": "string", "maxLength": 128}
			props["values"].(schemaObject)["items"] = schemaObject{"type": "number", "minimum": minValue, "maximum": 1e100}
		}
		if t == reflect.TypeFor[ScatterChartProps]() {
			result["required"] = []string{"points"}
			props["points"].(schemaObject)["maxItems"] = 4096
		}
		if t == reflect.TypeFor[ScatterPoint]() {
			result["required"] = []string{"x", "y"}
			for _, name := range []string{"x", "y"} {
				props[name] = schemaObject{"type": "number", "minimum": -1e100, "maximum": 1e100}
			}
			props["label"].(schemaObject)["maxLength"] = 128
		}
		if t == reflect.TypeFor[LineChartOptions]() || t == reflect.TypeFor[CartesianChartOptions]() {
			for _, name := range []string{"gridLines", "xTicks"} {
				props[name] = schemaObject{"type": "integer", "anyOf": []any{schemaObject{"const": 0}, schemaObject{"type": "integer", "minimum": 2, "maximum": 12}}}
			}
			for name, limits := range map[string]schemaObject{
				"plotPadding":   {"type": "number", "minimum": 0, "maximum": 128},
				"yAxisWidth":    {"type": "number", "minimum": 0, "maximum": 512},
				"valueDecimals": {"type": "integer", "minimum": 0, "maximum": 6},
			} {
				props[name] = schemaObject{"anyOf": []any{limits, schemaObject{"type": "null"}}}
			}
			props["valuePrefix"].(schemaObject)["maxLength"] = 32
			props["valueSuffix"].(schemaObject)["maxLength"] = 32
		}
		if t == reflect.TypeFor[LineChartOptions]() {
			props["lineWidth"] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "exclusiveMinimum": 0, "maximum": 16}, schemaObject{"type": "null"}}}
		}
		if t == reflect.TypeFor[BarChartOptions]() || t == reflect.TypeFor[ScatterChartOptions]() || t == reflect.TypeFor[PieChartOptions]() {
			limit := 4096
			if t == reflect.TypeFor[PieChartOptions]() {
				limit = 128
			}
			props["colors"].(schemaObject)["maxItems"] = limit
		}
		if t == reflect.TypeFor[BarChartOptions]() {
			props["barWidth"] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "exclusiveMinimum": 0, "maximum": 1}, schemaObject{"type": "null"}}}
		}
		if t == reflect.TypeFor[PieChartOptions]() {
			props["innerRadius"] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": 0, "maximum": 0.9}, schemaObject{"type": "null"}}}
			props["valueDecimals"] = schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 0, "maximum": 6}, schemaObject{"type": "null"}}}
			props["valuePrefix"].(schemaObject)["maxLength"] = 32
			props["valueSuffix"].(schemaObject)["maxLength"] = 32
		}
		if t == reflect.TypeFor[ScatterChartOptions]() {
			props["pointRadius"] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": 1, "maximum": 16}, schemaObject{"type": "null"}}}
			props["xValueDecimals"] = schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 0, "maximum": 6}, schemaObject{"type": "null"}}}
			props["xValuePrefix"].(schemaObject)["maxLength"] = 32
			props["xValueSuffix"].(schemaObject)["maxLength"] = 32
		}
		if t == reflect.TypeFor[MorphPathProps]() {
			result["required"] = []string{"keyframes"}
			props["progress"] = schemaObject{"type": "number", "minimum": 0, "maximum": 1}
			props["keyframes"].(schemaObject)["minItems"] = 2
			props["keyframes"].(schemaObject)["maxItems"] = 18
			props["duration"] = schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 0, "maximum": 60000}, schemaObject{"type": "null"}}}
		}
		if t == reflect.TypeFor[MorphPathKeyframe]() {
			result["required"] = []string{"at", "path"}
			props["at"] = schemaObject{"type": "number", "minimum": 0, "maximum": 1}
			props["path"].(schemaObject)["minItems"] = 1
			props["path"].(schemaObject)["maxItems"] = 128
		}
		if t == reflect.TypeFor[PathCommand]() {
			result["required"] = []string{"op"}
			props["values"].(schemaObject)["maxItems"] = 6
		}
		if t == reflect.TypeFor[Transition]() {
			result["required"] = []string{"properties"}
			properties := props["properties"].(schemaObject)
			properties["minItems"], properties["maxItems"], properties["uniqueItems"] = 1, 7, true
			props["duration"] = schemaObject{"anyOf": []any{schemaObject{"type": "integer", "minimum": 0, "maximum": 60000}, schemaObject{"type": "null"}}}
		}
		if t == reflect.TypeFor[SpanProps]() {
			props["timeline"] = schemaObject{"type": "null"}
			props["transition"] = schemaObject{"anyOf": []any{schemaObject{"type": "null"}, schemaObject{"allOf": []any{
				schemaType(reflect.TypeFor[Transition]()),
				schemaObject{"properties": schemaObject{"properties": schemaObject{"items": schemaObject{"enum": []string{"opacity", "backgroundColor"}}}}},
			}}}}
		}
		if menu, ok := props["contextMenu"].(schemaObject); ok {
			menu["maxItems"] = 500
		}
		if shortcuts, ok := props["shortcuts"].(schemaObject); ok {
			shortcuts["maxItems"] = 128
		}
		if t == reflect.TypeFor[Shortcut]() {
			result["required"] = []string{"key", "onPress"}
			props["key"].(schemaObject)["minLength"] = 1
			props["onPress"] = schemaType(reflect.TypeFor[Handler]())
		}
		if t == reflect.TypeFor[ContextMenuItem]() {
			result["required"] = []string{"label", "onPress"}
			props["label"].(schemaObject)["minLength"] = 1
			props["onPress"] = schemaType(reflect.TypeFor[Handler]())
		}
		// Bounds mirror prepare; relationships such as min < max still need Core validation.
		if t == reflect.TypeFor[StateStyle]() {
			props["opacity"] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": 0, "maximum": 1}, schemaObject{"type": "null"}}}
		}
		if t == reflect.TypeFor[Style]() {
			props["opacity"] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": 0, "maximum": 1}, schemaObject{"type": "null"}}}
			for _, name := range []string{"scale", "rotate"} {
				props[name] = schemaObject{"anyOf": []any{schemaObject{"type": "number", "minimum": -16384, "maximum": 16384}, schemaObject{"type": "null"}}}
			}
			for name, raw := range props {
				s := raw.(schemaObject)
				if s["type"] == "number" {
					s["minimum"], s["maximum"] = 0, 16384
				}
				if name == "top" || name == "left" || name == "right" || name == "bottom" {
					s["minimum"], s["maximum"] = 0, 16384
				}
				if name == "aspectRatio" || name == "borderTopWidth" || name == "borderRightWidth" || name == "borderBottomWidth" || name == "borderLeftWidth" {
					bounds := s["anyOf"].([]any)[0].(schemaObject)
					bounds["maximum"] = 16384
					if name == "aspectRatio" {
						bounds["exclusiveMinimum"] = 0
					} else {
						bounds["minimum"] = 0
					}
				}
				if name == "maxLines" {
					s["minimum"], s["maximum"] = 0, 10000
				}
			}
		}
		if t == reflect.TypeFor[GridProps]() {
			result["required"] = []string{"columns"}
			props["columns"] = schemaObject{"type": "integer", "minimum": 1, "maximum": MaxNodes}
			props["gap"] = schemaType(reflect.TypeFor[Size]())
		}
		if t == reflect.TypeFor[ScrollProps]() {
			props["endThreshold"] = schemaType(reflect.TypeFor[Size]())
		}
		if t == reflect.TypeFor[LazyListProps]() {
			result["required"] = []string{"itemHeight"}
			props["itemHeight"] = schemaObject{"allOf": []any{schemaType(reflect.TypeFor[Size]()), schemaObject{"not": schemaObject{"enum": []any{0, nil}}}}}
		}
		if t == reflect.TypeFor[TextFieldProps]() {
			props["maxLines"] = schemaObject{"type": "integer", "minimum": 0, "maximum": 100}
			props["changeDebounce"] = schemaObject{"type": "integer", "minimum": 0, "maximum": 60000}
		}
		if t == reflect.TypeFor[SpacerProps]() {
			props["size"] = schemaType(reflect.TypeFor[Size]())
		}
		if t == reflect.TypeFor[SliderProps]() {
			props["step"] = schemaObject{"type": "number", "minimum": 0}
		}
		if t == reflect.TypeFor[NumberFieldProps]() {
			props["step"] = schemaObject{"type": "number", "minimum": 0}
		}
		if t == reflect.TypeFor[TableProps]() {
			result["required"] = []string{"columns"}
			props["columns"].(schemaObject)["minItems"] = 1
			props["columns"].(schemaObject)["maxItems"] = 64
			props["rows"].(schemaObject)["maxItems"] = 1000
		}
		if t == reflect.TypeFor[TableColumn]() || t == reflect.TypeFor[TableRow]() || t == reflect.TypeFor[TableAction]() {
			result["required"] = []string{"key"}
			props["key"] = schemaObject{"type": "string", "minLength": 1, "maxLength": 256}
		}
		if t == reflect.TypeFor[TableColumn]() {
			props["width"] = schemaType(reflect.TypeFor[Size]())
		}
		if t == reflect.TypeFor[TableRow]() {
			result["required"] = []string{"key", "cells"}
			props["cells"].(schemaObject)["maxItems"] = 64
			props["actions"].(schemaObject)["maxItems"] = 32
		}
		if t == reflect.TypeFor[TableAction]() {
			result["required"] = []string{"key", "label", "onPress"}
			props["label"].(schemaObject)["minLength"] = 1
			props["onPress"] = schemaType(reflect.TypeFor[Handler]())
		}
		if t == reflect.TypeFor[LinkProps]() {
			result["required"] = []string{"url"}
			props["url"] = schemaObject{"type": "string", "pattern": `^(https?://[^/?#\s]+|mailto:[^\s]+)`}
		}
		if t == reflect.TypeFor[ChoiceProps]() || t == reflect.TypeFor[MultiSelectProps]() {
			props["options"].(schemaObject)["maxItems"] = 500
		}
		if t == reflect.TypeFor[MultiSelectProps]() {
			props["value"] = schemaObject{"type": "array", "maxItems": 500, "uniqueItems": true, "items": schemaObject{"type": "string", "minLength": 1}}
		}
		if t == reflect.TypeFor[TabsProps]() {
			props["items"].(schemaObject)["maxItems"] = 500
		}
		if t == reflect.TypeFor[ChoiceOption]() {
			result["required"] = []string{"value"}
			props["value"].(schemaObject)["minLength"] = 1
		}
		if t == reflect.TypeFor[IconButtonProps]() {
			result["required"] = []string{"icon"}
			props["icon"].(schemaObject)["minLength"] = 1
		}
		if t == reflect.TypeFor[ImageProps]() {
			result["oneOf"] = []any{
				schemaObject{"required": []string{"icon"}, "properties": schemaObject{"icon": schemaObject{"type": "string", "minLength": 1}, "source": schemaObject{"type": "null"}}},
				schemaObject{"required": []string{"source"}, "properties": schemaObject{"source": schemaObject{"type": "object"}, "icon": schemaObject{"const": ""}, "iconColorMode": schemaObject{"const": ""}}},
			}
		}
		if t == reflect.TypeFor[ImageSource]() {
			result["required"] = []string{"imageType", "imageData"}
			props["imageType"] = schemaObject{"enum": []string{"base64", "svg", "absolute"}}
			props["imageData"].(schemaObject)["minLength"] = 1
		}
		return result
	default:
		panic("unsupported schema type: " + t.String())
	}
}
