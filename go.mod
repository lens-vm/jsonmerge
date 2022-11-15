module github.com/lens-vm/jsonmerge

go 1.18

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/stretchr/testify v1.8.1
	github.com/valyala/fastjson v1.6.3
)

require (
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/valyala/fastjson => github.com/lens-vm/fastjson v1.6.4-0.20221115105011-96124e899e4a
