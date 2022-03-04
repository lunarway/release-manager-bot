module github.com/lunarway/release-manager-bot

go 1.14

require (
	github.com/google/go-github/v32 v32.0.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79
	github.com/palantir/go-baseapp v0.3.1
	github.com/palantir/go-githubapp v0.12.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/rs/zerolog v1.26.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
)

replace github.com/google/go-github/v32 => github.com/lunarway/go-github/v32 v32.1.1-0.20200825071958-1d202be51fc6
