module csd-devtrack/backend

go 1.24.4

require (
	csd-devtrack/cli v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	gopkg.in/yaml.v3 v3.0.1
)

replace csd-devtrack/cli => ../cli
