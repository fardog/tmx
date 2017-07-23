# tmx

[![Build Status](https://travis-ci.org/fardog/tmx.svg?branch=master)](https://travis-ci.org/fardog/tmx)
[![GoDoc](https://godoc.org/github.com/fardog/tmx?status.svg)](http://godoc.org/github.com/fardog/tmx)
[![Go Report Card](https://goreportcard.com/badge/github.com/fardog/tmx)](https://goreportcard.com/report/github.com/fardog/tmx)


A parser for the [TMX][] file format, used in the [Tiled Map Editor][tiled].

## Installation

```
go get -u github.com/fardog/tmx
```

## Version Compatibility

This library has no dependencies outside of the Go standard library, and is
tested on Go 1.7 and above. It does not yet respect any versioning standards, so
you are encouraged to vendor it in your projects to ensure compatibility.

## TODO/Help Wanted

* CSV data formats are untested and use a custom parser
* Test Coverage is very poor

## License

MIT. See [LICENSE](./LICENSE) for details.

[TMX]: http://doc.mapeditor.org/reference/tmx-map-format/
[tiled]: http://www.mapeditor.org/
