# A mock MOCA server.

[![Tests](https://github.com/castingcode/mocka/actions/workflows/ci-test.yml/badge.svg)](https://github.com/castingcode/mocka/actions/workflows/ci-test.yml)
[![codecov](https://codecov.io/gh/castingcode/mocka/graph/badge.svg?token=C7EUFCEHED)](https://codecov.io/gh/castingcode/mocka)
[![Go Report Card](https://goreportcard.com/badge/github.com/castingcode/mocka)](https://goreportcard.com/report/github.com/castingcode/mocka)


## Summary

This module creates an HTTP server that accepts MOCA requests and returns canned MOCA responses. 
It is designed for mocking a MOCA server when testing other software that interfaces with a MOCA server. 
The server supports login and logout functionalities. 
For any SQL or Groovy requests, it performs an exact match to find the mock response. 
If no exact match is found for requests with local syntax, it will look for a match based on the first verb/noun pair, 
ignoring the where clause or any subsequent commands. The server allows for user-specific responses, 
enabling variations in the mock responses you create.


## Installation

To install the module, use the following command:

```sh
go get github.com/castingcode/mocka
```

## Usage

Import the module in your Go code:

```go
import "github.com/castingcode/mocka"
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License.