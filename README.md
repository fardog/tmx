# tmx

A parser for the [TMX][] file format, used in the [Tiled Map Editor][tiled].

## Installation

```
go get -u github.com/fardog/tmx
```

## Version Compatibility

This library has no dependencies outside of the Go standard library, and is
tested on Go 1.7 and above. It does not yet respect any versioning standards, so
you are encouraged to vendor it in your projects to ensure compatibility.

## Notes

* CSV Formats are not currently supported for layer data
* Not all features are currently tested

## License

MIT. See [LICENSE](./LICENSE) for details.

[TMX]: http://doc.mapeditor.org/reference/tmx-map-format/
[tiled]: http://www.mapeditor.org/
