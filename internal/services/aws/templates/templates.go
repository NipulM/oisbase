package templates

import (
	"embed"
	"io/fs"
)

//go:embed lambda/*.tmpl
var LambdaFS embed.FS

//go:embed dynamodb/*.tmpl
var DynamoDBFS embed.FS

//go:embed common/*.tmpl
var CommonFS embed.FS

//go:embed api-gw/http/*.tmpl
var APIGatewayFS embed.FS

//go:embed sqs/*.tmpl
var SQSFS embed.FS

// GetFS returns the embedded filesystem that contains files matching the glob.
func GetFS(glob string) fs.FS {
	for _, f := range []fs.FS{APIGatewayFS, LambdaFS, DynamoDBFS, CommonFS} {
		if matches, _ := fs.Glob(f, glob); len(matches) > 0 {
			return f
		}
	}
	return nil
}