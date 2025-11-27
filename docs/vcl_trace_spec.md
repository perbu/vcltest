# VCL_trace Log Line Specification

## Format

```
<configname> <trace_index> <source_index>.<line>.<column>
```

Example: `vcl1 42 0.15.5`

## Field Descriptions

| Field | Example | Description |
|-------|---------|-------------|
| **configname** | `vcl1` | The VCL configuration name (what you see in `vcl.list`, set via `vcl.load name ...`) |
| **trace_index** | `42` | A sequential counter incremented at each trace point in the compiled VCL. Useful for correlating with `VPI_count` coverage data. Stored in `vpi_ref[].cnt` |
| **source_index** | `0` | Index into the `srcname[]` array - identifies *which* VCL source file. `0` is typically your main VCL, `1` might be the builtin.vcl, higher numbers are `include`d files |
| **line** | `15` | Line number within that source file (1-indexed) |
| **column** | `5` | Column position on that line (1-indexed, tabs count as 8 spaces aligned) |

## Source Index Details

Sources are numbered in the order they're processed:
- `0` = Your main VCL file
- `1` = Usually `builtin.vcl` (appended automatically)
- `2+` = Any `include` statements, in order encountered

You can see the source names via the `srcname[]` array in the compiled C code, or by looking at VCL_Log/error messages which reference these indices.

## What Gets Traced

Trace points are emitted at:
1. Entry to compound blocks (`{`) - the logged position is the **first token inside** the block, not the brace itself
2. After if/elseif chains that lack a final `else`

Individual statements like `set`, `return`, `call` do **not** generate separate trace points.

## Enabling Tracing

Tracing can be enabled via:
- Feature flag at startup: `-p feature=+trace` (enables globally)
- VCL variable: `set req.trace = true` or `set bereq.trace = true` (per-request control)

## Implementation References

- Log emission: `bin/varnishd/cache/cache_vpi.c` (`VPI_trace()`)
- Trace point generation: `lib/libvcc/vcc_parse.c` (the `C()` macro)
- Reference table: `lib/libvcc/vcc_compile.c` (`vpi_ref` struct generation)
- VSL tag definition: `include/tbl/vsl_tags.h`
