
# OpenAPI Code Generator with BadgerDB Integration

A command-line tool to generate Go server code from OpenAPI v3 specifications, with integrated BadgerDB for persistent storage. This tool automates the creation of HTTP handlers for CRUD operations (GET, POST, PUT, DELETE) that interact with BadgerDB, and provides an interactive UI using Bubble Tea for ease of use.

## Overview

This tool parses an OpenAPI JSON specification and generates Go code including:
- **Structs** from schemas (both component and inline).
- **HTTP Server and Handlers** for defined endpoints with BadgerDB operations:
  - **GET**: Retrieve data from BadgerDB.
  - **POST**: Insert data into BadgerDB.
  - **PUT**: Update data in BadgerDB.
  - **DELETE**: Remove data from BadgerDB.
- **Database Utilities** for initializing and managing BadgerDB connections.
- **Main Entry Point** to start the server with graceful shutdown.
- **Go Module File** (`go.mod`) with necessary dependencies.

The tool features an interactive terminal UI built with Bubble Tea, allowing users to generate code, clean up generated folders, and create sample OpenAPI JSON files effortlessly.

## Features

- **Interactive UI**: Built with Bubble Tea for a user-friendly terminal experience.
- **BadgerDB Integration**: Persistent storage for API data using key-value pairs with entity prefixes to simulate tables.
- **CRUD Operations**: Automatically maps HTTP methods to database operations.
- **Sample JSON Generation**: Create a sample OpenAPI specification for testing.
- **Cleanup Command**: Easily delete generated code folders.
- **Modular Output**: Generates organized Go files (`models.go`, `server.go`, `handlers.go`, `db_util.go`, `db_init.go`, `main.go`, `go.mod`).

## Prerequisites

- **Go**: Version 1.18 or higher is required to build and run the tool and generated code.
- **Dependencies**: The tool requires `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/lipgloss` for the UI. The generated code requires `github.com/dgraph-io/badger/v3` for database operations.

## Installation

1. **Clone or Download**: Obtain the source code for the OpenAPI Code Generator.
   ```bash
   git clone <repository-url>
   cd openapi-generator
   ```

2. **Fetch Dependencies**: Install the required Go modules for the tool.
   ```bash
   go get github.com/charmbracelet/bubbletea
   go get github.com/charmbracelet/lipgloss
   ```

3. **Build the Tool**: Compile the generator binary.
   ```bash
   go build -o oapi-gen
   ```

## Usage

Run the tool to access the interactive UI and perform various operations.

```bash
./oapi-gen
```

### Interactive Menu Options

- **Generate Code from OpenAPI Spec**:
  - Prompts for the path to your OpenAPI JSON specification file.
  - Prompts for the output directory for generated code (defaults to `generated`).
  - Generates Go server code with BadgerDB integration.

- **Clean Up Generated Folder**:
  - Confirms deletion of the generated code folder (uses specified output directory or defaults to `generated`).
  - Removes the folder and all its contents.

- **Generate Sample OpenAPI JSON**:
  - Prompts for the output file path for the sample JSON (defaults to `./sample-openapi.json`).
  - Creates a sample OpenAPI specification file for testing purposes.

- **Exit**:
  - Quits the application.

### Navigation

- Use **arrow keys** to navigate menu options.
- Press **Enter** to select an option or confirm input.
- Press **q** or **Ctrl+C** to quit the application at any time.

### Running the Generated Server

After generating code, you can run the server directly from the output directory:

1. **Navigate to the Output Directory**:
   ```bash
   cd generated
   ```

2. **Fetch Dependencies**:
   ```bash
   go mod tidy
   ```

3. **Run the Server**:
   ```bash
   go run .
   ```
   The server starts on `:8080` by default and uses BadgerDB for data storage in `./badger_db`.

### Testing CRUD Operations

Use tools like `curl` to interact with the generated API endpoints. For example, with the sample OpenAPI spec:

- **Create a User (POST)**:
  ```bash
  curl -X POST http://localhost:8080/users -H "Content-Type: application/json" -d '{"name": "John", "age": 30}'
  ```

- **Retrieve a User (GET)**:
  Replace `{id}` with the ID returned from POST or use a query parameter `id`.
  ```bash
  curl http://localhost:8080/users/{id}
  ```

- **Update a User (PUT)**:
  ```bash
  curl -X PUT http://localhost:8080/users/{id} -H "Content-Type: application/json" -d '{"name": "John Updated", "age": 31}'
  ```

- **Delete a User (DELETE)**:
  ```bash
  curl -X DELETE http://localhost:8080/users/{id}
  ```

**Note**: ID generation in the generated code is simplistic (timestamp-based). For production, consider replacing it with UUID or another unique identifier system.

## Sample OpenAPI JSON

The "Generate Sample OpenAPI JSON" option creates a file with a basic user management API specification, including endpoints for listing, creating, updating, and deleting users. You can use this file as input to test the code generation feature.

## Limitations

- **ID Generation**: The generated code uses a timestamp-based ID for new records. Replace with a UUID library or similar for production use.
- **Path Parameters**: Assumes IDs are in the URL path or query parameters. Complex parameter structures may require additional parsing logic.
- **BadgerDB Configuration**: Uses default settings. Tune options like memory usage or sync behavior for production environments.
- **Input Validation**: Basic UI input handling without advanced validation or autocompletion. Enhance as needed for robustness.

## Contributing

Contributions are welcome! Please submit issues or pull requests to improve functionality, fix bugs, or add features. Ensure any changes are tested with various OpenAPI specifications.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.

## Contact

For questions or support, please open an issue on the repository or contact the maintainers.

---

*Generated with ❤️ by the OpenAPI Code Generator Team*
