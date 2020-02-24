## FiCo Search
- Processing streams with GoLang

### Running Locally
- Be in project root dir
- Install Deps: `go install`
- Build: `go build` -> creates executable file
- Run: `go run app.go --file PATH/TO/FILE` (or use `-f` instead of `-- file`)
- Custom timeout arg: `go run app.go --file PATH/TO/FILE --timeout CUSTOM_TIME` (or use `-t` instead of `--timeout`)

### Running UNIX Shell Executable
- From the CLI in the dir that contains `fico-search`, run `fico-search -f PATH/TO/FILE`
  - ie. `fico-search ./samples/test.txt`
- Set a custom timeout by adding the `-t` flag. ie. `fico-search -f PATH/TO/FILE -t 120`
  - Timeout defaults to 60s.
