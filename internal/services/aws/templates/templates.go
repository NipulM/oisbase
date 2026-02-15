package templates

import "embed"

//go:embed lambda/*.tmpl
var LambdaFS embed.FS

//go:embed dynamodb/*.tmpl
var DynamoDBFS embed.FS

//go:embed common/*.tmpl
var CommonFS embed.FS