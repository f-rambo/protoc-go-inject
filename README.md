# protoc-go-inject

A Go tool for automatically enhancing protobuf-generated Go files with custom annotations.

## Features

- Process custom annotations in protobuf-generated Go files
- Add new package imports with `@goimport`
- Add new struct fields with `@gofield`
- Append or modify struct field tags with `@gotags`
- Case-insensitive field name matching
- Preserves original file structure and comments

## Installation

Requires Go 1.20 or higher.

```bash
# Clone the repository
git clone https://github.com/f-rambo/protoc-go-inject.git

# Build and install
cd protoc-go-inject
make install
```

## Usage

```bash
# Process a single file
protoc-go-inject path/to/generated.pb.go

# Process multiple files
protoc-go-inject file1.pb.go file2.pb.go

# Show help
protoc-go-inject -h
```

## Annotation Examples

In your protobuf file:

```protobuf
message User {
    // @goimport: "gorm.io/gorm"
    // @gofield: gorm.Model
    // @gofield: LastName string
    string id = 1; // @gotags: gorm:"column:id;primaryKey;AUTO_INCREMENT"
    string name = 2; // @gotags: gorm:"column:name;type:varchar(255)"
}
```

## Supported Annotations

- `@goimport`: Add new package imports
  ```
  // @goimport: "gorm.io/gorm"
  ```

- `@gofield`: Add new struct fields
  ```
  // @gofield: gorm.Model
  // @gofield: LastName string
  ```

- `@gotags`: Append or modify struct field tags
  ```
  // @gotags: gorm:"column:id;primaryKey" json:"id"
  ```

## Development

### Prerequisites

- Go 1.20+
- Make

### Build Commands

```bash
# Build the binary
make build

# Install to GOPATH/bin
make install

# Clean build artifacts
make clean

# Show available commands
make help
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the  Apache 2.0 License - see the LICENSE file for details.

## Acknowledgments

- Built with Go's AST manipulation tools
- Inspired by the need for automated protobuf file enhancement