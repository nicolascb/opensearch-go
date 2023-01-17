module github.com/nicolascb/opensearch/v2/internal/build

go 1.15

replace github.com/nicolascb/opensearch/v2 => ../../

require (
	github.com/alecthomas/chroma v0.8.2
	github.com/kr/pretty v0.1.0 // indirect
	github.com/nicolascb/opensearch/v2 v2.1.0
	github.com/spf13/cobra v1.6.1
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/tools v0.1.12
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.4.0
)
