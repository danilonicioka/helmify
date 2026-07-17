# Template Function List

> **Warning**: This page has not yet been updated for Helm 4. Some of the content might be inaccurate or not applicable to Helm 4. For more information about the Helm 4 new features, improvements, and breaking changes, see **Helm 4 Overview**.

Helm includes many template functions you can take advantage of in templates. They are listed here and broken down by the following categories:

- **Cryptographic and Security**
- **Date**
- **Dictionaries**
- **Encoding**
- **File Path**
- **Kubernetes and Chart**
- **Logic and Flow Control**
- **Lists**
- **Math**
- **Float Math**
- **Network**
- **Reflection**
- **Regular Expressions**
- **Semantic Versions**
- **String**
- **Type Conversion**
- **URL**
- **UUID**

---

## Logic and Flow Control Functions

Helm includes numerous logic and control flow functions, including:

- `all`
- `and`
- `any`
- `or`
- `not`
- `eq`
- `ne`
- `lt`
- `le`
- `gt`
- `ge`
- `default`
- `empty`
- `required`
- `fail`
- `coalesce`
- `ternary`

### `all`

Returns true if **all** of the given values are non‑empty.

```gotemplate
all .Arg1 .Arg2 .Arg3
```

### `and`

Returns the boolean AND of two or more arguments (the first empty argument, or the last argument).

```gotemplate
and .Arg1 .Arg2
```

### `any`

Returns true if **any** of the given values are non‑empty.

```gotemplate
any .Arg1 .Arg2 .Arg3
```

### `or`

Returns the boolean OR of two or more arguments (the first non‑empty argument, or the last argument).

```gotemplate
or .Arg1 .Arg2
```

### `not`

Returns the boolean negation of its argument.

```gotemplate
not .Arg
```

### `eq` / `ne`

Equality and inequality checks.

```gotemplate
eq .Arg1 .Arg2
ne .Arg1 .Arg2
```

### `lt` / `le` / `gt` / `ge`

Numeric comparisons.

```gotemplate
lt .Arg1 .Arg2   # less than
le .Arg1 .Arg2   # less or equal
gt .Arg1 .Arg2   # greater than
ge .Arg1 .Arg2   # greater or equal
```

### `default`

Provide a fallback value when the given value is empty.

```gotemplate
default "foo" .Bar
```

### `empty`

Returns true if the supplied value is considered empty (zero, empty string, empty list/dict, false, or nil).

```gotemplate
empty .Foo
```

### `required`

Ensures a value is set; otherwise the template rendering fails with the supplied message.

```gotemplate
required "A valid foo is required!" .Bar
```

### `fail`

Unconditionally aborts rendering with an error message.

```gotemplate
fail "Please accept the end user license agreement"
```

### `coalesce`

Returns the first non‑empty argument.

```gotemplate
coalesce .name .parent.name "Matt"
```

### `ternary`

Conditional expression; returns the first value if the test is true, otherwise the second.

```gotemplate
ternary "foo" "bar" true   # returns "foo"
ternary "foo" "bar" false  # returns "bar"
```

Or using pipeline syntax:

```gotemplate
true | ternary "foo" "bar"
```

---

## String Functions

Helm provides a rich set of string manipulation functions:

- `abbrev`, `abbrevboth`, `camelcase`, `cat`, `contains`, `hasPrefix`, `hasSuffix`, `indent`, `initials`, `kebabcase`, `lower`, `nindent`, `nospace`, `plural`, `print`, `printf`, `println`, `quote`, `randAlpha`, `randAlphaNum`, `randAscii`, `randNumeric`, `repeat`, `replace`, `shuffle`, `snakecase`, `squote`, `substr`, `swapcase`, `title`, `trim`, `trimAll`, `trimPrefix`, `trimSuffix`, `trunc`, `untitle`, `upper`, `wrap`, `wrapWith`.

### `print` and `println`

Combine arguments into a string (adds spaces between non‑string arguments). `println` adds a newline.

```gotemplate
print "Matt has " .Dogs " dogs"
println "Done"
```

### `printf`

Formatted output using Go format verbs.

```gotemplate
printf "%s has %d dogs." .Name .NumberDogs
```

### `quote` / `squote`

Wrap a string in double or single quotes.

```gotemplate
quote "Hello"
```

### `upper` / `lower`

Change case.

```gotemplate
upper "hello"   # HELLO
lower "HELLO"   # hello
```

### `trim`, `trimAll`, `trimPrefix`, `trimSuffix`

Whitespace and character trimming utilities.

```gotemplate
trim "   hello   "
trimAll "$" "$5.00"
trimPrefix "-" "-hello"
trimSuffix "-" "hello-"
```

### `repeat`

Repeat a string N times.

```gotemplate
repeat 3 "hello"   # hellohellohello
```

### `substr`

Extract a substring (start, end, string).

```gotemplate
substr 0 5 "hello world"   # hello
```

### `wrap` / `wrapWith`

Wrap text at a column count, optionally with a custom delimiter.

```gotemplate
wrap 80 $someText
wrapWith 5 "\t" "Hello World"
```

---

## Type Conversion Functions

- `atoi`, `float64`, `int`, `int64`, `toDecimal`, `toString`, `toStrings`
- JSON/YAML/TOML: `toJson`, `mustToJson`, `toPrettyJson`, `mustToPrettyJson`, `toRawJson`, `mustToRawJson`, `toYaml`, `toYamlPretty`, `toToml`, `mustToToml`
- Parsing: `fromYaml`, `fromJson`, `fromJsonArray`, `fromYamlArray`

Example:

```gotemplate
{{ $obj := .Files.Get "config.yaml" | fromYaml }}
{{ $obj | toJson }}
```

---

## Date Functions

- `now`
- `ago`
- `date`
- `dateInZone`
- `duration`
- `durationRound`
- `unixEpoch`
- `dateModify` / `mustDateModify`
- `htmlDate`
- `htmlDateInZone`
- `toDate` / `mustToDate`

Example:

```gotemplate
{{ now | date "2006-01-02" }}
{{ now | dateModify "-1h" | date "15:04" }}
```

---

## Math Functions (int)

`add`, `add1`, `sub`, `div`, `mod`, `mul`, `max`, `min`, `len`, `randInt`, `ceil`, `floor`, `round`

Example:

```gotemplate
{{ add 1 2 3 }}   # 6
{{ mul 2 3 }}    # 6
{{ randInt 12 30 }}
```

## Float Math Functions

`addf`, `add1f`, `subf`, `divf`, `mulf`, `maxf`, `minf`, `floor`, `ceil`, `round`

---

## List Functions

Create and manipulate immutable lists:

- `list`
- `append` / `mustAppend`
- `prepend` / `mustPrepend`
- `concat`
- `first` / `mustFirst`
- `last` / `mustLast`
- `rest` / `mustRest`
- `initial` / `mustInitial`
- `reverse` / `mustReverse`
- `uniq` / `mustUniq`
- `without` / `mustWithout`
- `has` / `mustHas`
- `compact` / `mustCompact`
- `chunk`
- `seq`
- `until`
- `untilStep`
- `slice` / `mustSlice`
- `index`

Example:

```gotemplate
{{ $list := list 1 2 3 }}
{{ $list | append 4 | uniq }}
```

---

## Dictionary (dict) Functions

- `dict`
- `get`
- `set`
- `unset`
- `hasKey`
- `keys`
- `values`
- `merge` / `mustMerge`
- `mergeOverwrite` / `mustMergeOverwrite`
- `pick`
- `omit`
- `dig`
- `pluck`
- `deepCopy` / `mustDeepCopy`

Example:

```gotemplate
{{ $d := dict "name" "bob" "age" 30 }}
{{ $d | set "city" "Paris" }}
{{ $d | get "name" }}
```

---

## Encoding Functions

- `b64enc`, `b64dec`
- `b32enc`, `b32dec`

---

## Network Functions

- `getHostByName` (requires `--enable-dns` flag on Helm)

---

## File Path Functions

- `base`, `clean`, `dir`, `ext`, `isAbs`

---

## Reflection Functions

- `kindOf`, `kindIs`
- `typeOf`, `typeIs`, `typeIsLike`
- `deepEqual`

---

## Regular Expression Functions

- `regexMatch`, `mustRegexMatch`
- `regexFind`, `mustRegexFind`
- `regexFindAll`, `mustRegexFindAll`
- `regexReplaceAll`, `mustRegexReplaceAll`
- `regexReplaceAllLiteral`, `mustRegexReplaceAllLiteral`
- `regexSplit`, `mustRegexSplit`

---

## Semantic Version Functions

- `semver`
- `semverCompare`

---

## URL Functions

- `urlParse`
- `urlJoin`
- `urlquery`

---

## UUID Functions

- `uuidv4`

---

## Kubernetes and Chart Functions

- `.Capabilities.APIVersions.Has`
- `lookup`
- `Files` (e.g., `Files.Get`, `Files.Glob`, `Files.AsSecrets`)

---

## Checklist
- Review the function categories relevant to your chart.
- Prefer pipeline syntax (`|`) for readability.
- Use `default` and `required` to enforce sensible defaults.
- Remember that `lookup` requires a live cluster and will return empty during `--dry-run`.
- When using cryptographic functions, avoid hard‑coding secrets in templates.

This page serves as a quick reference for the many functions Helm makes available to template authors.
