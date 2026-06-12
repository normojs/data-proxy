package openapi

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gopkg.in/yaml.v3"
)

type Spec struct {
	OpenAPIUrl string
	Title      string
	Version    string
	ServerURL  string
	Operations []Operation
}

type Operation struct {
	Key                 string
	OperationId         string
	Method              string
	Path                string
	Summary             string
	Description         string
	ServerURL           string
	InputSchema         map[string]any
	Parameters          []Parameter
	RequestBodySchema   map[string]any
	RequestContentType  string
	ResponseContentType string
}

type Parameter struct {
	Name     string         `json:"name"`
	In       string         `json:"in"`
	Required bool           `json:"required"`
	Schema   map[string]any `json:"schema"`
}

var safeNamePattern = regexp.MustCompile(`[^a-z0-9_-]+`)

const maxRefResolveDepth = 32

func ParseSpec(data []byte, sourceURL string) (*Spec, error) {
	var doc map[string]any
	if err := common.Unmarshal(data, &doc); err != nil {
		if yamlErr := yaml.Unmarshal(data, &doc); yamlErr != nil {
			return nil, fmt.Errorf("invalid OpenAPI JSON/YAML document: %w", err)
		}
	}
	if doc == nil {
		return nil, fmt.Errorf("openapi document is empty")
	}
	resolver := newRefResolver(doc)
	spec := &Spec{
		OpenAPIUrl: strings.TrimSpace(sourceURL),
		Title:      stringValue(mapValue(doc, "info"), "title"),
		Version:    stringValue(mapValue(doc, "info"), "version"),
		ServerURL:  firstServerURL(doc, sourceURL),
	}
	paths := mapValue(doc, "paths")
	if len(paths) == 0 {
		return nil, fmt.Errorf("openapi document has no paths")
	}
	pathKeys := make([]string, 0, len(paths))
	for key := range paths {
		pathKeys = append(pathKeys, key)
	}
	sort.Strings(pathKeys)
	for _, pathName := range pathKeys {
		pathItem := resolver.resolveMap(paths[pathName])
		if len(pathItem) == 0 {
			continue
		}
		commonParameters := parseParameters(sliceValue(pathItem["parameters"]), resolver)
		methods := make([]string, 0, len(pathItem))
		for key := range pathItem {
			if isHTTPMethod(key) {
				methods = append(methods, strings.ToLower(key))
			}
		}
		sort.Strings(methods)
		for _, method := range methods {
			operationMap := resolver.resolveMap(pathItem[method])
			if len(operationMap) == 0 {
				continue
			}
			operation := parseOperation(spec.ServerURL, pathName, method, operationMap, commonParameters, resolver)
			spec.Operations = append(spec.Operations, operation)
		}
	}
	if len(spec.Operations) == 0 {
		return nil, fmt.Errorf("openapi document has no supported operations")
	}
	return spec, nil
}

func parseOperation(serverURL string, pathName string, method string, operationMap map[string]any, commonParameters []Parameter, resolver *refResolver) Operation {
	operationId := strings.TrimSpace(stringValue(operationMap, "operationId"))
	if operationId == "" {
		operationId = method + "_" + sanitizeName(pathName)
	}
	parameters := make([]Parameter, 0, len(commonParameters))
	parameters = append(parameters, commonParameters...)
	parameters = append(parameters, parseParameters(sliceValue(operationMap["parameters"]), resolver)...)
	requestSchema, requestContentType := parseRequestBody(operationMap, resolver)
	responseContentType := parseResponseContentType(operationMap)
	key := strings.ToUpper(method) + " " + pathName
	return Operation{
		Key:                 key,
		OperationId:         operationId,
		Method:              strings.ToUpper(method),
		Path:                pathName,
		Summary:             strings.TrimSpace(stringValue(operationMap, "summary")),
		Description:         strings.TrimSpace(stringValue(operationMap, "description")),
		ServerURL:           serverURL,
		InputSchema:         buildInputSchema(parameters, requestSchema, requestContentType),
		Parameters:          dedupeParameters(parameters),
		RequestBodySchema:   requestSchema,
		RequestContentType:  requestContentType,
		ResponseContentType: responseContentType,
	}
}

func parseParameters(items []any, resolver *refResolver) []Parameter {
	parameters := make([]Parameter, 0, len(items))
	for _, item := range items {
		value := resolver.resolveMap(item)
		if len(value) == 0 {
			continue
		}
		name := strings.TrimSpace(stringValue(value, "name"))
		location := strings.TrimSpace(stringValue(value, "in"))
		if name == "" || location == "" {
			continue
		}
		parameters = append(parameters, Parameter{
			Name:     name,
			In:       location,
			Required: boolValue(value, "required"),
			Schema:   schemaValue(value["schema"], resolver),
		})
	}
	return dedupeParameters(parameters)
}

func parseRequestBody(operationMap map[string]any, resolver *refResolver) (map[string]any, string) {
	requestBody := resolver.resolveMap(operationMap["requestBody"])
	if len(requestBody) == 0 {
		return nil, ""
	}
	content := mapValue(requestBody, "content")
	if len(content) == 0 {
		return map[string]any{"type": "object"}, "application/json"
	}
	contentType := preferredRequestContentType(content)
	media := mapAny(content[contentType])
	schema := schemaValue(media["schema"], resolver)
	if len(schema) == 0 {
		schema = map[string]any{"type": "object"}
	}
	if boolValue(requestBody, "required") {
		schema["x-openapi-required"] = true
	}
	return schema, contentType
}

func preferredRequestContentType(content map[string]any) string {
	for _, contentType := range []string{
		"application/json",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
		"text/plain",
	} {
		if _, ok := content[contentType]; ok {
			return contentType
		}
	}
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "application/json"
	}
	return keys[0]
}

func parseResponseContentType(operationMap map[string]any) string {
	responses := mapValue(operationMap, "responses")
	preferred := []string{"200", "201", "202", "default"}
	for _, code := range preferred {
		if contentType := responseContentType(responses[code]); contentType != "" {
			return contentType
		}
	}
	keys := make([]string, 0, len(responses))
	for key := range responses {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if contentType := responseContentType(responses[key]); contentType != "" {
			return contentType
		}
	}
	return ""
}

func responseContentType(response any) string {
	content := mapValue(mapAny(response), "content")
	if len(content) == 0 {
		return ""
	}
	if _, ok := content["application/json"]; ok {
		return "application/json"
	}
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func buildInputSchema(parameters []Parameter, requestSchema map[string]any, requestContentType string) map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, parameter := range dedupeParameters(parameters) {
		schema := cloneMap(parameter.Schema)
		if len(schema) == 0 {
			schema = map[string]any{"type": "string"}
		}
		schema["description"] = strings.TrimSpace(fmt.Sprintf("%s parameter", parameter.In))
		properties[parameter.Name] = schema
		if parameter.Required || parameter.In == "path" {
			required = append(required, parameter.Name)
		}
	}
	if len(requestSchema) > 0 {
		bodySchema := cloneMap(requestSchema)
		delete(bodySchema, "x-openapi-required")
		bodySchema = inputSchemaForRequestBody(bodySchema, requestContentType)
		properties["body"] = bodySchema
		if requestSchema["x-openapi-required"] == true {
			required = append(required, "body")
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}
	return schema
}

func inputSchemaForRequestBody(schema map[string]any, contentType string) map[string]any {
	mediaType, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(contentType)), ";")
	if schemaIsBinary(schema) || mediaType == "application/octet-stream" {
		return binaryRequestBodyInputSchema(schema)
	}
	if mediaType == "multipart/form-data" {
		return multipartInputSchema(schema)
	}
	return schema
}

func multipartInputSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return schema
	}
	if schemaIsBinary(schema) {
		return fileUploadInputSchema(schema)
	}
	normalized := cloneMap(schema)
	if properties := mapValue(normalized, "properties"); len(properties) > 0 {
		normalizedProperties := map[string]any{}
		for key, value := range properties {
			normalizedProperties[key] = multipartInputSchema(mapAny(value))
		}
		normalized["properties"] = normalizedProperties
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if child := mapAny(normalized[key]); len(child) > 0 {
			normalized[key] = multipartInputSchema(child)
		}
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if items := sliceValue(normalized[key]); len(items) > 0 {
			converted := make([]any, 0, len(items))
			for _, item := range items {
				converted = append(converted, multipartInputSchema(mapAny(item)))
			}
			normalized[key] = converted
		}
	}
	return normalized
}

func binaryRequestBodyInputSchema(original map[string]any) map[string]any {
	description := strings.TrimSpace(stringValue(original, "description"))
	if description == "" {
		description = "Binary request body"
	}
	return map[string]any{
		"type":        "object",
		"description": description + ". Provide optional content_type and base64 body content.",
		"required":    []string{"content_base64"},
		"properties": map[string]any{
			"content_type": map[string]any{
				"type":        "string",
				"description": "Optional request Content-Type override. Defaults to the OpenAPI media type.",
			},
			"content_base64": map[string]any{
				"type":        "string",
				"description": "Base64-encoded request body content.",
			},
		},
		"x-openapi-binary-body": true,
	}
}

func fileUploadInputSchema(original map[string]any) map[string]any {
	description := strings.TrimSpace(stringValue(original, "description"))
	if description == "" {
		description = "Multipart file field"
	}
	return map[string]any{
		"type":        "object",
		"description": description + ". Provide filename, optional content_type, and base64 file content.",
		"required":    []string{"content_base64"},
		"properties": map[string]any{
			"filename": map[string]any{
				"type":        "string",
				"description": "File name sent in the multipart Content-Disposition filename parameter.",
			},
			"content_type": map[string]any{
				"type":        "string",
				"description": "Optional file Content-Type. Defaults to application/octet-stream.",
			},
			"content_base64": map[string]any{
				"type":        "string",
				"description": "Base64-encoded file content.",
			},
		},
		"x-openapi-file-upload": true,
	}
}

func schemaIsBinary(schema map[string]any) bool {
	schemaType := strings.ToLower(strings.TrimSpace(fmt.Sprint(schema["type"])))
	format := strings.ToLower(strings.TrimSpace(fmt.Sprint(schema["format"])))
	return schemaType == "file" || (schemaType == "string" && (format == "binary" || format == "base64"))
}

func ToolName(namespace string, operation Operation) string {
	name := sanitizeName(operation.OperationId)
	if name == "" {
		name = sanitizeName(operation.Method + "_" + operation.Path)
	}
	namespace = sanitizeName(namespace)
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "{", "_")
	value = strings.ReplaceAll(value, "}", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = safeNamePattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_-")
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	if len(value) > 64 {
		value = strings.Trim(value[:64], "_-")
	}
	return value
}

func firstServerURL(doc map[string]any, sourceURL string) string {
	servers := sliceValue(doc["servers"])
	if len(servers) > 0 {
		server := mapAny(servers[0])
		if value := strings.TrimSpace(stringValue(server, "url")); value != "" {
			return resolveServerURL(value, sourceURL)
		}
	}
	if sourceURL == "" {
		return ""
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func resolveServerURL(serverURL string, sourceURL string) string {
	if serverURL == "" {
		return ""
	}
	parsed, err := url.Parse(serverURL)
	if err == nil && parsed.IsAbs() {
		return serverURL
	}
	if sourceURL == "" {
		return serverURL
	}
	base, err := url.Parse(sourceURL)
	if err != nil {
		return serverURL
	}
	if strings.HasPrefix(serverURL, "/") {
		return base.Scheme + "://" + base.Host + serverURL
	}
	base.Path = path.Dir(base.Path) + "/"
	return base.ResolveReference(&url.URL{Path: serverURL}).String()
}

func isHTTPMethod(value string) bool {
	switch strings.ToLower(value) {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	default:
		return false
	}
}

func dedupeParameters(parameters []Parameter) []Parameter {
	result := make([]Parameter, 0, len(parameters))
	seen := map[string]bool{}
	for _, parameter := range parameters {
		key := parameter.In + ":" + parameter.Name
		if seen[key] || parameter.Name == "" || parameter.In == "" {
			continue
		}
		seen[key] = true
		result = append(result, parameter)
	}
	return result
}

func schemaValue(value any, resolver *refResolver) map[string]any {
	var schema map[string]any
	if resolver != nil {
		schema = resolver.resolveSchema(value)
	} else {
		schema = mapAny(value)
	}
	if len(schema) == 0 {
		return map[string]any{"type": "string"}
	}
	return cloneMap(schema)
}

type refResolver struct {
	doc map[string]any
}

func newRefResolver(doc map[string]any) *refResolver {
	return &refResolver{doc: doc}
}

func (r *refResolver) resolveMap(value any) map[string]any {
	if r == nil {
		return cloneMap(mapAny(value))
	}
	return r.resolveReferenceMap(mapAny(value), map[string]bool{}, 0)
}

func (r *refResolver) resolveSchema(value any) map[string]any {
	schema := r.resolveMap(value)
	if len(schema) == 0 {
		return schema
	}
	return r.normalizeSchema(schema, map[string]bool{}, 0)
}

func (r *refResolver) resolveReferenceMap(input map[string]any, seen map[string]bool, depth int) map[string]any {
	if len(input) == 0 {
		return nil
	}
	if depth > maxRefResolveDepth {
		return cloneMap(input)
	}
	current := cloneMap(input)
	if ref := strings.TrimSpace(stringValue(current, "$ref")); ref != "" {
		target := r.resolveRef(ref, seen, depth+1)
		if len(target) > 0 {
			merged := cloneMap(target)
			for key, value := range current {
				if key == "$ref" {
					continue
				}
				merged[key] = value
			}
			current = merged
		}
	}
	for key, value := range current {
		switch typed := value.(type) {
		case map[string]any:
			current[key] = r.resolveReferenceMap(typed, seen, depth+1)
		case []any:
			items := make([]any, 0, len(typed))
			for _, item := range typed {
				if itemMap := mapAny(item); len(itemMap) > 0 {
					items = append(items, r.resolveReferenceMap(itemMap, seen, depth+1))
				} else {
					items = append(items, item)
				}
			}
			current[key] = items
		}
	}
	return current
}

func (r *refResolver) resolveRef(ref string, seen map[string]bool, depth int) map[string]any {
	if r == nil || depth > maxRefResolveDepth || !strings.HasPrefix(ref, "#/") {
		return nil
	}
	if seen[ref] {
		return nil
	}
	target := jsonPointerValue(r.doc, strings.TrimPrefix(ref, "#"))
	targetMap := mapAny(target)
	if len(targetMap) == 0 {
		return nil
	}
	seen[ref] = true
	resolved := r.resolveReferenceMap(targetMap, seen, depth+1)
	delete(seen, ref)
	return resolved
}

func (r *refResolver) normalizeSchema(schema map[string]any, seen map[string]bool, depth int) map[string]any {
	if len(schema) == 0 || depth > maxRefResolveDepth {
		return schema
	}
	resolved := r.resolveReferenceMap(schema, seen, depth+1)
	if len(resolved) == 0 {
		return schema
	}
	if allOf := sliceValue(resolved["allOf"]); len(allOf) > 0 {
		merged := map[string]any{}
		for _, item := range allOf {
			itemSchema := r.normalizeSchema(mapAny(item), seen, depth+1)
			merged = mergeSchemas(merged, itemSchema)
		}
		for key, value := range resolved {
			if key == "allOf" || key == "$ref" {
				continue
			}
			merged[key] = value
		}
		resolved = merged
	}
	if properties := mapValue(resolved, "properties"); len(properties) > 0 {
		normalizedProperties := map[string]any{}
		for key, value := range properties {
			normalizedProperties[key] = r.normalizeSchema(mapAny(value), seen, depth+1)
		}
		resolved["properties"] = normalizedProperties
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if child := mapAny(resolved[key]); len(child) > 0 {
			resolved[key] = r.normalizeSchema(child, seen, depth+1)
		}
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		if items := sliceValue(resolved[key]); len(items) > 0 {
			normalized := make([]any, 0, len(items))
			for _, item := range items {
				normalized = append(normalized, r.normalizeSchema(mapAny(item), seen, depth+1))
			}
			resolved[key] = normalized
		}
	}
	return resolved
}

func mergeSchemas(base map[string]any, next map[string]any) map[string]any {
	result := cloneMap(base)
	if len(next) == 0 {
		return result
	}
	for key, value := range next {
		switch key {
		case "properties":
			properties := cloneMap(mapValue(result, "properties"))
			for propKey, propValue := range mapAny(value) {
				existingProperty := mapAny(properties[propKey])
				nextProperty := mapAny(propValue)
				if len(existingProperty) > 0 && len(nextProperty) > 0 {
					properties[propKey] = mergeSchemas(existingProperty, nextProperty)
				} else {
					properties[propKey] = propValue
				}
			}
			result[key] = properties
		case "required":
			result[key] = mergeStringSlices(sliceValue(result[key]), sliceValue(value))
		case "items", "additionalProperties":
			existingChild := mapAny(result[key])
			nextChild := mapAny(value)
			if len(existingChild) > 0 && len(nextChild) > 0 {
				result[key] = mergeSchemas(existingChild, nextChild)
			} else {
				result[key] = value
			}
		default:
			result[key] = value
		}
	}
	return result
}

func mergeStringSlices(left []any, right []any) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range append(left, right...) {
		value, ok := item.(string)
		if !ok || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func jsonPointerValue(root any, pointer string) any {
	if pointer == "" {
		return root
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil
	}
	current := root
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil
			}
			current = typed[index]
		default:
			return nil
		}
		if current == nil {
			return nil
		}
	}
	return current
}

func mapValue(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	return mapAny(parent[key])
}

func mapAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[any]any:
		result := map[string]any{}
		for key, value := range typed {
			result[fmt.Sprint(key)] = value
		}
		return result
	default:
		return nil
	}
}

func sliceValue(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

func stringValue(parent map[string]any, key string) string {
	if parent == nil {
		return ""
	}
	value, ok := parent[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func boolValue(parent map[string]any, key string) bool {
	if parent == nil {
		return false
	}
	switch value := parent[key].(type) {
	case bool:
		return value
	default:
		return false
	}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	bytes, err := common.Marshal(input)
	if err != nil {
		return input
	}
	var output map[string]any
	if err := common.Unmarshal(bytes, &output); err != nil {
		return input
	}
	return output
}
