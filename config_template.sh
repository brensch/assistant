#!/bin/sh
echo "Generating config template..."
# Run the generator
go run ./config/gen/main.go

exit 0