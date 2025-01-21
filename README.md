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

## Why? What does mocka do?

Mocka is not a tool to test your MOCA server.
It is intended to be used to test the things you create to interface with your MOCA server.
Note that the scope of mocka is simply to provide a canned response for a given request
which you can use in tests of your code. It can not be used to validate any side effects
produced as a result of your code.

For example, if you have a dashboard that executes a query in MOCA to display current inventory levels,
if you want to test that against an actual MOCA server, you will either need to account for the data
not being consistent in your tests. Or you can handle setup and tear down to get the data into a known
state for your test. With mocka, it just provides an easy way to get the same response every time you
send a specific request, allowing you to write integration tests without reduced overhead around test data
maintenance. It allows you to easily validate you're handling deserialization correctly, things like handling
date formatting, handling nulls, etc.

What mocka does not do is try to emulate any of the side effects resulting from the execution of your 
MOCA queries.  Beyond returning a result set, mocka makes no attempt to emulate any other behavior of your
MOCA server. If your testing needs are such that you need to validate that your command resulted in the
correct data being written to the correct tables, the proper transactions were sent to any other systems,
the proper triggers were fired, the proper labels or reports were generated, or any other type of side
effect resulting from your executed query, mocka is not the tool for you.

In other words, if you are writing some code that would log into a MOCA server and execute a "move inventory"
command, and you wanted to validate your code handles various sorts of responses, such as various failures
or varying result sets due to moves at various inventory levels, you could use mocka for that. If you needed
to validate the inventory was successfully moved and the proper transactions were logged, you'll need to 
use a real MOCA server or look for another tool.

This does not replace any end-to-end testing with an actual MOCA server. It's just intended to make testing
the quick and simple cases quickly and simply.

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
Note these are not the full responses from the server.
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