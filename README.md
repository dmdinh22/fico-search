## FiCo Search
- Processing streams with GoLang

### Running Locally
- Be in project root dir
- Install Deps: `go install`
- Build: `go build` -> creates executable file
- Run: `go run app.go --file PATH/TO/FILE` (or use `-f` instead of `-- file`)
- Custom timeout arg: `go run app.go --file PATH/TO/FILE --timeout CUSTOM_TIME` (or use `-t` instead of `--timeout`)

### Building executable for different OS (amd64 platform)
- Inside the `src/` dir where `app.go` lives, run the following command
- MacOS (Darwin)
  - `GOOS=darwin GOARCH=amd64 go build -o fico-search app.go`
- Linux
  - `GOOS=linux GOARCH=amd64 go build -o fico-search app.go`
- Windows
  - `GOOS=windows GOARCH=amd64 go build -o fico-search.exe app.go`

### Running UNIX Shell Executable
- From the CLI in the dir that contains `fico-search`, run `./fico-search -f PATH/TO/FILE`
  - ie. `./fico-search ./samples/test.txt`
- Set a custom timeout by adding the `-t` flag. ie. `./fico-search -f PATH/TO/FILE -t 120`
  - Timeout defaults to 60s.
