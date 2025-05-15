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

// Schema represents a schema definition in components/schemas or inline
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
	// Generate structs from schemas (including inline schemas)
	schemas, _ := extractSchemas(spec.Components)
	// Extract additional schemas from paths (inline schemas)
	inlineSchemas := extractInlineSchemas(spec.Paths)
	// Merge schemas, prioritizing components over inline if names clash
	for k, v := range inlineSchemas {
		if _, exists := schemas[k]; !exists {
			schemas[k] = v
		}
	}
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

	// Generate database utility code for BadgerDB
	dbUtilCode := generateDBUtilCode(schemas)
	if err := writeFile(filepath.Join(outputDir, "db_util.go"), dbUtilCode); err != nil {
		return err
	}

	// Generate database initialization code
	dbInitCode := generateDBInitCode(schemas)
	if err := writeFile(filepath.Join(outputDir, "db_init.go"), dbInitCode); err != nil {
		return err
	}

	// Generate main.go to tie everything together
	mainCode := generateMainCode()
	if err := writeFile(filepath.Join(outputDir, "main.go"), mainCode); err != nil {
		return err
	}

	// Generate go.mod file with BadgerDB dependency
	goModCode := generateGoModCode()
	if err := writeFile(filepath.Join(outputDir, "go.mod"), goModCode); err != nil {
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

// extractInlineSchemas extracts inline schema definitions from paths
func extractInlineSchemas(paths map[string]interface{}) map[string]Schema {
	schemas := make(map[string]Schema)
	for path, methodsRaw := range paths {
		methods, _ := methodsRaw.(map[string]interface{})
		for method, endpointRaw := range methods {
			endpoint, _ := endpointRaw.(map[string]interface{})
			operationID, _ := endpoint["operationId"].(string)
			if operationID == "" {
				operationID = fmt.Sprintf("%s%s", strings.ToUpper(method), strings.ReplaceAll(path, "/", ""))
			}

			// Check requestBody for inline schema
			if reqBody, ok := endpoint["requestBody"].(map[string]interface{}); ok {
				if content, ok := reqBody["content"].(map[string]interface{}); ok {
					if appJSON, ok := content["application/json"].(map[string]interface{}); ok {
						if schemaRaw, ok := appJSON["schema"].(map[string]interface{}); ok {
							if _, hasRef := schemaRaw["$ref"]; !hasRef {
								schemaJSON, _ := json.Marshal(schemaRaw)
								var schema Schema
								json.Unmarshal(schemaJSON, &schema)
								schemaName := fmt.Sprintf("%sRequest", toGoIdentifier(strings.ReplaceAll(operationID, " ", "")))
								schemas[schemaName] = schema
							}
						}
					}
				}
			}

			// Check responses for inline schema
			if responses, ok := endpoint["responses"].(map[string]interface{}); ok {
				for status, respRaw := range responses {
					if resp, ok := respRaw.(map[string]interface{}); ok {
						if content, ok := resp["content"].(map[string]interface{}); ok {
							if appJSON, ok := content["application/json"].(map[string]interface{}); ok {
								if schemaRaw, ok := appJSON["schema"].(map[string]interface{}); ok {
									if _, hasRef := schemaRaw["$ref"]; !hasRef {
										schemaJSON, _ := json.Marshal(schemaRaw)
										var schema Schema
										json.Unmarshal(schemaJSON, &schema)
										schemaName := fmt.Sprintf("%sResponse%s", toGoIdentifier(strings.ReplaceAll(operationID, " ", "")), status)
										schemas[schemaName] = schema
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return schemas
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
	serverCode.WriteString("import (\n    \"fmt\"\n    \"log\"\n    \"net/http\"\n    \"github.com/dgraph-io/badger/v3\"\n)\n\n")
	serverCode.WriteString("var DB *badger.DB\n\n")
	serverCode.WriteString("func StartServer(db *badger.DB) {\n")
	serverCode.WriteString("    DB = db\n")
	serverCode.WriteString("    mux := http.NewServeMux()\n")

	handlerCode.WriteString("package main\n\n")
	handlerCode.WriteString("import (\n    \"encoding/json\"\n    \"fmt\"\n    \"net/http\"\n    \"strings\"\n    \"github.com/dgraph-io/badger/v3\"\n)\n\n")

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

			// Derive entity name from path for BadgerDB key prefix
			entityName := deriveEntityName(path)
			keyPrefix := fmt.Sprintf("%s:", entityName)

			// Handle different HTTP methods with BadgerDB operations
			switch strings.ToUpper(method) {
			case "GET":
				// GET: Retrieve from BadgerDB
				handlerCode.WriteString("    // Extract ID from URL path if applicable\n")
				handlerCode.WriteString(fmt.Sprintf("    pathParts := strings.Split(r.URL.Path, \"/\")\n"))
				handlerCode.WriteString("    var id string\n")
				handlerCode.WriteString("    if len(pathParts) > 2 {\n")
				handlerCode.WriteString("        id = pathParts[len(pathParts)-1]\n")
				handlerCode.WriteString("    } else {\n")
				handlerCode.WriteString("        id = r.URL.Query().Get(\"id\")\n")
				handlerCode.WriteString("    }\n")
				handlerCode.WriteString("    if id == \"\" {\n")
				handlerCode.WriteString("        http.Error(w, \"ID not provided\", http.StatusBadRequest)\n")
				handlerCode.WriteString("        return\n    }\n")
				handlerCode.WriteString(fmt.Sprintf("    key := fmt.Sprintf(\"%s%%s\", id)\n", keyPrefix))
				handlerCode.WriteString("    var result []byte\n")
				handlerCode.WriteString("    err := DB.View(func(txn *badger.Txn) error {\n")
				handlerCode.WriteString("        item, err := txn.Get([]byte(key))\n")
				handlerCode.WriteString("        if err != nil {\n")
				handlerCode.WriteString("            return err\n")
				handlerCode.WriteString("        }\n")
				handlerCode.WriteString("        result, err = item.ValueCopy(nil)\n")
				handlerCode.WriteString("        return err\n")
				handlerCode.WriteString("    })\n")
				handlerCode.WriteString("    if err == badger.ErrKeyNotFound {\n")
				handlerCode.WriteString(fmt.Sprintf("        http.Error(w, \"%s not found\", http.StatusNotFound)\n", entityName))
				handlerCode.WriteString("        return\n")
				handlerCode.WriteString("    } else if err != nil {\n")
				handlerCode.WriteString("        http.Error(w, \"Database error\", http.StatusInternalServerError)\n")
				handlerCode.WriteString("        return\n")
				handlerCode.WriteString("    }\n")
				// Handle response struct
				responses, _ := endpoint["responses"].(map[string]interface{})
				successResp, _ := responses["200"].(map[string]interface{})
				if content, ok := successResp["content"].(map[string]interface{}); ok {
					appJSON, _ := content["application/json"].(map[string]interface{})
					schemaRef, _ := appJSON["schema"].(map[string]interface{})
					ref, _ := schemaRef["$ref"].(string)
					var structName string
					if ref != "" {
						structName = toGoIdentifier(strings.TrimPrefix(ref, "#/components/schemas/"))
					} else {
						structName = fmt.Sprintf("%sResponse200", handlerName)
					}
					if structName != "" {
						handlerCode.WriteString(fmt.Sprintf("    var resp %s\n", structName))
						handlerCode.WriteString("    if err := json.Unmarshal(result, &resp); err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Failed to parse data\", http.StatusInternalServerError)\n")
						handlerCode.WriteString("        return\n")
						handlerCode.WriteString("    }\n")
						handlerCode.WriteString("    w.Header().Set(\"Content-Type\", \"application/json\")\n")
						handlerCode.WriteString("    json.NewEncoder(w).Encode(resp)\n")
					} else {
						handlerCode.WriteString("    fmt.Fprintf(w, string(result))\n")
					}
				} else {
					handlerCode.WriteString("    fmt.Fprintf(w, string(result))\n")
				}

			case "POST":
				// POST: Insert into BadgerDB
				if reqBody, ok := endpoint["requestBody"].(map[string]interface{}); ok {
					content, _ := reqBody["content"].(map[string]interface{})
					appJSON, _ := content["application/json"].(map[string]interface{})
					schemaRef, _ := appJSON["schema"].(map[string]interface{})
					ref, _ := schemaRef["$ref"].(string)
					var structName string
					if ref != "" {
						structName = toGoIdentifier(strings.TrimPrefix(ref, "#/components/schemas/"))
					} else {
						structName = fmt.Sprintf("%sRequest", handlerName)
					}
					if structName != "" {
						handlerCode.WriteString(fmt.Sprintf("    var reqBody %s\n", structName))
						handlerCode.WriteString("    if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Invalid request body\", http.StatusBadRequest)\n")
						handlerCode.WriteString("        return\n    }\n")
						// Generate a simple ID (in production, use UUID or similar)
						handlerCode.WriteString("    id := fmt.Sprintf(\"%d\", time.Now().UnixNano())\n")
						handlerCode.WriteString("    data, err := json.Marshal(reqBody)\n")
						handlerCode.WriteString("    if err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Failed to serialize data\", http.StatusInternalServerError)\n")
						handlerCode.WriteString("        return\n    }\n")
						handlerCode.WriteString(fmt.Sprintf("    key := fmt.Sprintf(\"%s%%s\", id)\n", keyPrefix))
						handlerCode.WriteString("    err = DB.Update(func(txn *badger.Txn) error {\n")
						handlerCode.WriteString("        return txn.Set([]byte(key), data)\n")
						handlerCode.WriteString("    })\n")
						handlerCode.WriteString("    if err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Failed to save data\", http.StatusInternalServerError)\n")
						handlerCode.WriteString("        return\n    }\n")
						// Handle response
						responses, _ := endpoint["responses"].(map[string]interface{})
						successResp, _ := responses["200"].(map[string]interface{})
						if content, ok := successResp["content"].(map[string]interface{}); ok {
							appJSON, _ := content["application/json"].(map[string]interface{})
							schemaRef, _ := appJSON["schema"].(map[string]interface{})
							ref, _ := schemaRef["$ref"].(string)
							var respStructName string
							if ref != "" {
								respStructName = toGoIdentifier(strings.TrimPrefix(ref, "#/components/schemas/"))
							} else {
								respStructName = fmt.Sprintf("%sResponse200", handlerName)
							}
							if respStructName != "" {
								handlerCode.WriteString(fmt.Sprintf("    resp := %s{}\n", respStructName))
								handlerCode.WriteString("    // TODO: Populate response fields as needed\n")
								handlerCode.WriteString("    w.Header().Set(\"Content-Type\", \"application/json\")\n")
								handlerCode.WriteString("    json.NewEncoder(w).Encode(resp)\n")
							} else {
								handlerCode.WriteString("    fmt.Fprintf(w, \"Data saved with ID: \" + id)\n")
							}
						} else {
							handlerCode.WriteString("    fmt.Fprintf(w, \"Data saved with ID: \" + id)\n")
						}
					}
				}

			case "PUT":
				// PUT: Update in BadgerDB
				handlerCode.WriteString("    // Extract ID from URL path if applicable\n")
				handlerCode.WriteString("    pathParts := strings.Split(r.URL.Path, \"/\")\n")
				handlerCode.WriteString("    var id string\n")
				handlerCode.WriteString("    if len(pathParts) > 2 {\n")
				handlerCode.WriteString("        id = pathParts[len(pathParts)-1]\n")
				handlerCode.WriteString("    } else {\n")
				handlerCode.WriteString("        id = r.URL.Query().Get(\"id\")\n")
				handlerCode.WriteString("    }\n")
				handlerCode.WriteString("    if id == \"\" {\n")
				handlerCode.WriteString("        http.Error(w, \"ID not provided\", http.StatusBadRequest)\n")
				handlerCode.WriteString("        return\n    }\n")
				if reqBody, ok := endpoint["requestBody"].(map[string]interface{}); ok {
					content, _ := reqBody["content"].(map[string]interface{})
					appJSON, _ := content["application/json"].(map[string]interface{})
					schemaRef, _ := appJSON["schema"].(map[string]interface{})
					ref, _ := schemaRef["$ref"].(string)
					var structName string
					if ref != "" {
						structName = toGoIdentifier(strings.TrimPrefix(ref, "#/components/schemas/"))
					} else {
						structName = fmt.Sprintf("%sRequest", handlerName)
					}
					if structName != "" {
						handlerCode.WriteString(fmt.Sprintf("    var reqBody %s\n", structName))
						handlerCode.WriteString("    if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Invalid request body\", http.StatusBadRequest)\n")
						handlerCode.WriteString("        return\n    }\n")
						handlerCode.WriteString("    data, err := json.Marshal(reqBody)\n")
						handlerCode.WriteString("    if err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Failed to serialize data\", http.StatusInternalServerError)\n")
						handlerCode.WriteString("        return\n    }\n")
						handlerCode.WriteString(fmt.Sprintf("    key := fmt.Sprintf(\"%s%%s\", id)\n", keyPrefix))
						handlerCode.WriteString("    err = DB.Update(func(txn *badger.Txn) error {\n")
						handlerCode.WriteString("        return txn.Set([]byte(key), data)\n")
						handlerCode.WriteString("    })\n")
						handlerCode.WriteString("    if err != nil {\n")
						handlerCode.WriteString("        http.Error(w, \"Failed to update data\", http.StatusInternalServerError)\n")
						handlerCode.WriteString("        return\n    }\n")
						handlerCode.WriteString("    fmt.Fprintf(w, \"Data updated for ID: \" + id)\n")
					}
				}

			case "DELETE":
				// DELETE: Remove from BadgerDB
				handlerCode.WriteString("    // Extract ID from URL path if applicable\n")
				handlerCode.WriteString("    pathParts := strings.Split(r.URL.Path, \"/\")\n")
				handlerCode.WriteString("    var id string\n")
				handlerCode.WriteString("    if len(pathParts) > 2 {\n")
				handlerCode.WriteString("        id = pathParts[len(pathParts)-1]\n")
				handlerCode.WriteString("    } else {\n")
				handlerCode.WriteString("        id = r.URL.Query().Get(\"id\")\n")
				handlerCode.WriteString("    }\n")
				handlerCode.WriteString("    if id == \"\" {\n")
				handlerCode.WriteString("        http.Error(w, \"ID not provided\", http.StatusBadRequest)\n")
				handlerCode.WriteString("        return\n    }\n")
				handlerCode.WriteString(fmt.Sprintf("    key := fmt.Sprintf(\"%s%%s\", id)\n", keyPrefix))
				handlerCode.WriteString("    err := DB.Update(func(txn *badger.Txn) error {\n")
				handlerCode.WriteString("        return txn.Delete([]byte(key))\n")
				handlerCode.WriteString("    })\n")
				handlerCode.WriteString("    if err != nil {\n")
				handlerCode.WriteString("        http.Error(w, \"Failed to delete data\", http.StatusInternalServerError)\n")
				handlerCode.WriteString("        return\n    }\n")
				handlerCode.WriteString("    fmt.Fprintf(w, \"Data deleted for ID: \" + id)\n")

			default:
				handlerCode.WriteString("    http.Error(w, \"Unsupported method\", http.StatusMethodNotAllowed)\n")
			}
			handlerCode.WriteString("}\n\n")
		}
	}

	serverCode.WriteString("    fmt.Println(\"Server starting on :8080\")\n")
	serverCode.WriteString("    log.Fatal(http.ListenAndServe(\":8080\", mux))\n")
	serverCode.WriteString("}\n")

	return serverCode.String(), handlerCode.String()
}

// generateDBUtilCode creates utility functions for BadgerDB operations
func generateDBUtilCode(schemas map[string]Schema) string {
	var code strings.Builder
	code.WriteString("package main\n\n")
	code.WriteString("import (\n    \"log\"\n    \"github.com/dgraph-io/badger/v3\"\n)\n\n")
	code.WriteString("// InitializeDB sets up the BadgerDB connection\n")
	code.WriteString("func InitializeDB(dbPath string) (*badger.DB, error) {\n")
	code.WriteString("    opts := badger.DefaultOptions(dbPath)\n")
	code.WriteString("    opts.Logger = nil // Disable logging or customize as needed\n")
	code.WriteString("    db, err := badger.Open(opts)\n")
	code.WriteString("    if err != nil {\n")
	code.WriteString("        log.Printf(\"Failed to open BadgerDB: %v\", err)\n")
	code.WriteString("        return nil, err\n")
	code.WriteString("    }\n")
	code.WriteString("    return db, nil\n")
	code.WriteString("}\n\n")
	code.WriteString("// CloseDB closes the BadgerDB connection\n")
	code.WriteString("func CloseDB(db *badger.DB) {\n")
	code.WriteString("    if err := db.Close(); err != nil {\n")
	code.WriteString("        log.Printf(\"Failed to close BadgerDB: %v\", err)\n")
	code.WriteString("    }\n")
	code.WriteString("}\n")
	return code.String()
}

// generateDBInitCode creates initialization code for BadgerDB with "tables" as key prefixes
func generateDBInitCode(schemas map[string]Schema) string {
	var code strings.Builder
	code.WriteString("package main\n\n")
	code.WriteString("import (\n    \"fmt\"\n    \"log\"\n    \"github.com/dgraph-io/badger/v3\"\n)\n\n")
	code.WriteString("// SetupDB initializes the database with necessary prefixes or initial data\n")
	code.WriteString("func SetupDB(db *badger.DB) error {\n")
	code.WriteString("    // BadgerDB is a key-value store, so we simulate 'tables' with key prefixes\n")
	code.WriteString("    // Prefixes are used to organize data by entity type\n")
	code.WriteString("    prefixes := []string{\n")
	// Add a prefix for each schema or entity type
	for schemaName := range schemas {
		code.WriteString(fmt.Sprintf("        \"%s:\",\n", strings.ToLower(schemaName)))
	}
	code.WriteString("    }\n\n")
	code.WriteString("    // Optionally, initialize with dummy data or metadata\n")
	code.WriteString("    err := db.Update(func(txn *badger.Txn) error {\n")
	code.WriteString("        // Example: Add metadata or initial empty entries if needed\n")
	code.WriteString("        for _, prefix := range prefixes {\n")
	code.WriteString("            metaKey := fmt.Sprintf(\"%smetadata\", prefix)\n")
	code.WriteString("            if err := txn.Set([]byte(metaKey), []byte(\"initialized\")); err != nil {\n")
	code.WriteString("                return err\n")
	code.WriteString("            }\n")
	code.WriteString("        }\n")
	code.WriteString("        return nil\n")
	code.WriteString("    })\n")
	code.WriteString("    if err != nil {\n")
	code.WriteString("        log.Printf(\"Failed to setup database: %v\", err)\n")
	code.WriteString("        return err\n")
	code.WriteString("    }\n")
	code.WriteString("    log.Println(\"Database setup completed with prefixes for entities\")\n")
	code.WriteString("    return nil\n")
	code.WriteString("}\n")
	return code.String()
}

// generateMainCode creates the main entry point to initialize DB and start server
func generateMainCode() string {
	var code strings.Builder
	code.WriteString("package main\n\n")
	code.WriteString("import (\n    \"log\"\n    \"os\"\n    \"os/signal\"\n    \"syscall\"\n)\n\n")
	code.WriteString("func main() {\n")
	code.WriteString("    // Initialize BadgerDB\n")
	code.WriteString("    dbPath := \"./badger_db\"\n")
	code.WriteString("    db, err := InitializeDB(dbPath)\n")
	code.WriteString("    if err != nil {\n")
	code.WriteString("        log.Fatal(err)\n")
	code.WriteString("    }\n")
	code.WriteString("    defer CloseDB(db)\n\n")
	code.WriteString("    // Setup database with prefixes or initial data\n")
	code.WriteString("    if err := SetupDB(db); err != nil {\n")
	code.WriteString("        log.Fatal(err)\n")
	code.WriteString("    }\n\n")
	code.WriteString("    // Start HTTP server\n")
	code.WriteString("    go StartServer(db)\n\n")
	code.WriteString("    // Wait for interrupt signal to gracefully shutdown\n")
	code.WriteString("    sigChan := make(chan os.Signal, 1)\n")
	code.WriteString("    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)\n")
	code.WriteString("    <-sigChan\n")
	code.WriteString("    log.Println(\"Shutting down server...\")\n")
	code.WriteString("}\n")
	return code.String()
}

// generateGoModCode creates a go.mod file with the BadgerDB dependency
func generateGoModCode() string {
	var code strings.Builder
	code.WriteString("module generated\n\n")
	code.WriteString("go 1.23.8\n\n")
	code.WriteString("require github.com/dgraph-io/badger/v3 v3.2103.5\n")
	return code.String()
}

// deriveEntityName extracts a meaningful entity name from the path
func deriveEntityName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		return toGoIdentifier(parts[0])
	}
	return "Entity"
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
