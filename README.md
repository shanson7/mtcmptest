# Metrictank Comparison Testing Tool

This CLI tool compares series returned from Metrictank and proxy (Graphite). This is useful when writing native Metrictank functions.

## Usage

```
mtcmptest [-url] [-range] [-verbose] <files>
```

- `url` specifies the url of the query (e.g. `http://metrictank.host.com/render`)
- `range` is given in seconds. Translates to `from` and `until` parameters, where `until` is always current time
- `verbose` is set for more output about requests
- `files` is a list of json files in the following format:

```
{
  "test-name-1": "target1",
  "test-name-2": "target2",
}
```

## TODO

- [ ] Sort series to not fail tests when series are in different orders
- [ ] Sanitize tags to always be strings
- [ ] Add performance comparison
