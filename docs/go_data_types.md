# Appendix: Go Data Types and Templates

The Helm template language is implemented in the strongly typed Go programming language. For that reason, variables in templates are typed. For the most part, variables will be exposed as one of the following types:

- **string**: A string of text
- **bool**: A true or false value
- **int**: An integer value (there are also 8, 16, 32, and 64‑bit signed and unsigned variants)
- **float64**: A 64‑bit floating‑point value (there are also 8, 16, and 32‑bit varieties)
- **[]byte**: A byte slice, often used to hold (potentially) binary data
- **struct**: An object with properties and methods
- **slice**: An indexed list of one of the previous types (`[]string`, `[]int`, …)
- **map[string]interface{}**: A string‑keyed map where the value is one of the previous types

There are many other Go types, and sometimes you will need to convert between them in your templates. The easiest way to debug an object's type is to pass it through `printf "%T"` in a template, which will print the type. Helm also provides the `typeOf` and `kindOf` functions for introspection.

```yaml
# Example: printing a variable's type
myVar: {{ printf "%T" .Values.someValue }}
```

> **Tip:** Use `typeOf` to get the underlying Go reflect.Type and `kindOf` to retrieve the kind (e.g., `reflect.Struct`, `reflect.Slice`).

---
*This appendix is part of the Helmify documentation hub.*
