package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OpenAPISpec represents the structure of an OpenAPI v3 specification
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       map[string]interface{} `json:"info"`
	Paths      map[string]interface{} `json:"paths"`
	Components map[string]interface{} `json:"components"`
}

// Schema represents a schema definition in components/schemas
type Schema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Items      map[string]interface{} `json:"items,omitempty"`
	Ref        string                 `json:"$ref,omitempty"`
}

// Main function to handle CLI arguments and code generation
func main() {
	inputFile := flag.String("input", "", "Path to the OpenAPI JSON specification file")
	outputDir := flag.String("output", "generated", "Directory to output generated code")
	flag.Parse()

	if *inputFile == "" {
		fmt.Println("Error: Input file path is required. Use --input <path>")
		os.Exit(1)
	}

	// Read and parse the OpenAPI spec
	spec, err := readOpenAPISpec(*inputFile)
	if err != nil {
		fmt.Printf("Error reading OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Generate code
	if err := generateCode(spec, *outputDir); err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Code generated successfully in %s\n", *outputDir)
}

// readOpenAPISpec reads and unmarshals the OpenAPI JSON file
func readOpenAPISpec(filePath string) (*OpenAPISpec, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &spec, nil
}

// generateCode orchestrates the generation of structs and server code
func generateCode(spec *OpenAPISpec, outputDir string) error {
	// Generate structs from schemas
	schemas, _ := extractSchemas(spec.Components)
	structCode := generateStructs(schemas)
	if err := writeFile(filepath.Join(outputDir, "models.go"), structCode); err != nil {
		return err
	}

	// Generate server and handlers from paths
	serverCode, handlerCode := generateServerAndHandlers(spec.Paths, schemas)
	if err := writeFile(filepath.Join(outputDir, "server.go"), serverCode); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outputDir, "handlers.go"), handlerCode); err != nil {
		return err
	}

	return nil
}

// extractSchemas extracts schema definitions from components
func extractSchemas(components map[string]interface{}) (map[string]Schema, error) {
	schemas := make(map[string]Schema)
	schemaData, ok := components["schemas"]
	if !ok {
		return schemas, nil
	}

	schemaMap, ok := schemaData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid schemas format")
	}

	for name, raw := range schemaMap {
		schemaJSON, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var schema Schema
		if err := json.Unmarshal(schemaJSON, &schema); err != nil {
			return nil, err
		}
		schemas[name] = schema
	}
	return schemas, nil
}

// generateStructs creates Go struct definitions from schemas
func generateStructs(schemas map[string]Schema) string {
	var code strings.Builder
	code.WriteString("package main\n\n")
	code.WriteString("// Auto-generated structs from OpenAPI spec\n\n")

	for name, schema := range schemas {
		code.WriteString(fmt.Sprintf("type %s struct {\n", toGoIdentifier(name)))
		if schema.Type == "object" && schema.Properties != nil {
			for propName, propRaw := range schema.Properties {
				prop, _ := propRaw.(map[string]interface{})
				propType, _ := prop["type"].(string)
				code.WriteString(fmt.Sprintf("    %s %s `json:\"%s\"`\n", toGoIdentifier(propName), mapTypeToGo(propType), propName))
			}
		}
		code.WriteString("}\n\n")
	}
	return code.String()
}

// generateServerAndHandlers creates server setup and endpoint handlers
func generateServerAndHandlers(paths map[string]interface{}, schemas map[string]Schema) (string, string) {
	var serverCode, handlerCode strings.Builder

	serverCode.WriteString("package main\n\n")
	serverCode.WriteString("import (\n    \"fmt\"\n    \"log\"\n    \"net/http\"\n)\n\n")
	serverCode.WriteString("func StartServer() {\n")
	serverCode.WriteString("    mux := http.NewServeMux()\n")

	handlerCode.WriteString("package main\n\n")
	handlerCode.WriteString("import (\n    \"encoding/json\"\n    \"fmt\"\n    \"net/http\"\n)\n\n")

	for path, methodsRaw := range paths {
		methods, _ := methodsRaw.(map[string]interface{})
		for method, endpointRaw := range methods {
			endpoint, _ := endpointRaw.(map[string]interface{})
			operationID, _ := endpoint["operationId"].(string)
			if operationID == "" {
				operationID = fmt.Sprintf("%s%s", strings.ToUpper(method), strings.ReplaceAll(path, "/", ""))
			}

			// Remove spaces from operationId to ensure valid Go identifier
			handlerName := toGoIdentifier(strings.ReplaceAll(operationID, " ", ""))
			serverCode.WriteString(fmt.Sprintf("    mux.HandleFunc(\"%s\", %s)\n", path, handlerName))

			handlerCode.WriteString(fmt.Sprintf("func %s(w http.ResponseWriter, r *http.Request) {\n", handlerName))
			handlerCode.WriteString(fmt.Sprintf("    if r.Method != \"%s\" {\n", strings.ToUpper(method)))
			handlerCode.WriteString("        http.Error(w, \"Method not allowed\", http.StatusMethodNotAllowed)\n")
			handlerCode.WriteString("        return\n    }\n")

			// Handle request body if present
			if reqBody, ok := endpoint["requestBody"].(map[string]interface{}); ok {
				content, _ := reqBody["content"].(map[string]interface{})
				appJSON, _ := content["application/json"].(map[string]interface{})
				schemaRef, _ := appJSON["schema"].(map[string]interface{})
				ref, _ := schemaRef["$ref"].(string)
				if ref != "" {
					structName := toGoIdentifier(strings.TrimPrefix(ref, "#/components/schemas/"))
					handlerCode.WriteString(fmt.Sprintf("    var reqBody %s\n", structName))
					handlerCode.WriteString("    if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {\n")
					handlerCode.WriteString("        http.Error(w, \"Invalid request body\", http.StatusBadRequest)\n")
					handlerCode.WriteString("        return\n    }\n")
				}
			}

			// Handle responses
			responses, _ := endpoint["responses"].(map[string]interface{})
			successResp, _ := responses["200"].(map[string]interface{})
			if content, ok := successResp["content"].(map[string]interface{}); ok {
				appJSON, _ := content["application/json"].(map[string]interface{})
				schemaRef, _ := appJSON["schema"].(map[string]interface{})
				ref, _ := schemaRef["$ref"].(string)
				if ref != "" {
					structName := toGoIdentifier(strings.TrimPrefix(ref, "#/components/schemas/"))
					handlerCode.WriteString(fmt.Sprintf("    resp := %s{}\n", structName))
					handlerCode.WriteString("    // TODO: Implement response logic\n")
					handlerCode.WriteString("    w.Header().Set(\"Content-Type\", \"application/json\")\n")
					handlerCode.WriteString("    json.NewEncoder(w).Encode(resp)\n")
				} else {
					handlerCode.WriteString("    fmt.Fprintf(w, \"Operation completed\")\n")
				}
			} else {
				handlerCode.WriteString("    fmt.Fprintf(w, \"Operation completed\")\n")
			}
			handlerCode.WriteString("}\n\n")
		}
	}

	serverCode.WriteString("    fmt.Println(\"Server starting on :8080\")\n")
	serverCode.WriteString("    log.Fatal(http.ListenAndServe(\":8080\", mux))\n")
	serverCode.WriteString("}\n")

	return serverCode.String(), handlerCode.String()
}

// mapTypeToGo converts OpenAPI types to Go types
func mapTypeToGo(openAPIType string) string {
	switch openAPIType {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		return "[]interface{}"
	case "object":
		return "map[string]interface{}"
	default:
		return "interface{}"
	}
}

// toGoIdentifier converts a name to a valid Go identifier
func toGoIdentifier(name string) string {
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, " ", "")
	parts := strings.Split(name, "")
	if len(parts) > 0 {
		parts[0] = strings.ToUpper(parts[0])
	}
	return strings.Join(parts, "")
}

// writeFile writes content to a file
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
