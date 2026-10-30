# Contributing

Use Go 1.26.10 or later. Keep tests independent of production credentials and the private Vallkey service source.

```sh
go test -race ./...
go vet ./...
go build ./...
```

Run `gofmt` on changed Go files. Include regression coverage for changes to planning, import, drift detection, API errors, or state handling. Treat both provider credentials and Terraform state as sensitive. Update the resource or data-source documentation when schemas change.

For substantial changes, discuss the proposed behavior in an issue first. Submit focused pull requests with a clear problem statement and validation results. Contributions are accepted under the MIT license.
