# A mock MOCA server.

[![Tests](https://github.com/castingcode/mocka/actions/workflows/test.yml/badge.svg)](https://github.com/castingcode/mocka/actions/workflows/test.yml)
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

Alternatively, you can install the module using:

```sh
go install github.com/castingcode/mocka@latest
```

The pre-build binaries are available for download from https://github.com/castingcode/mocka/releases.

## Flags

The following flags are available in the main package:

- `-port`: Specifies the port on which the server will run. Default is `9000`.
- `-folder`: Specifies the path to the folder containing responses YAML files. The default is a "responses" folder where the executable resides.

## Responses YAML

The responses YAML file defines the mock responses for the server, where the command is the key, and the response as XML is the value.
Here are some examples:

```yaml
publish data where a = 'foo':
  status: 0
  results: >
    <moca-results>
        <metadata>
            <column name="a" type="I" length="0" nullable="true"/>
        </metadata>
        <data>
            <row>
                <field>foo</field>
            </row>
        </data>
    </moca-results>
publish data:
  status: 0
  results: >
    <moca-results>
        <metadata>
            <column name="line" type="I" length="0" nullable="true"/>
            <column name="text" type="S" length="0" nullable="true"/>
        </metadata>
        <data>
            <row>
                <field>0</field>
                <field>hello</field>
            </row>
        </data>
    </moca-results>
list warehouses:
  status: 510
  message: No Data Found
```

### Notes

- Groovy and SQL requests are matched exactly.
- Local syntax requests will first attempt an exact match. If no exact match is found, they will match based on the command name, ignoring any `where` clause or subsequent commands.
- Since the file is in YAML format, you need to enclose any Groovy or SQL keys in quotes and escape any quotes within the string.

## Usage

Import the module in your Go code:

```go
import "github.com/castingcode/mocka"
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License.