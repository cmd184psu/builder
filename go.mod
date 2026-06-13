module github.com/cmd184psu/builder

go 1.24.0

toolchain go1.24.7

require github.com/cmd184psu/alfredo v0.2.0

require (
	github.com/aws/aws-sdk-go v1.55.7 // indirect
	github.com/go-chi/chi/v5 v5.2.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/pkg/sftp v1.13.9 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/term v0.35.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)

replace github.com/cmd184psu/alfredo => ../alfredo
