# Reference fixtures

This directory holds the engineering fixture records introduced alongside tested
methods. The values live in Go test literals so that the core keeps no filesystem
dependency; these files are the record of where each value came from, at full
precision, with its tolerances and its discrepancies.

- [wing-planform-book-example.md](wing-planform-book-example.md) — the CODE Lab
  Wing Planform Sizing worked example, Task 03, plus the independent Reynolds
  fixture.
- [cg-book-example.md](cg-book-example.md) — the CODE Lab Center of gravity
  worked example, Task 07. Task 07's own placement fixture is independent of the
  book and is described in `calculator/mass_test.go`.

Task 02's lift fixtures are synthetic and are described in
[docs/reference/sources.md](../../docs/reference/sources.md).

Each fixture must identify its chapter/section URL, access date or upstream
revision, original units, full-precision inputs, expected values, physical and
relative tolerances, assumptions and adaptations. Preserve original-unit and SI
equivalence without using rounded intermediate outputs. Record discrepancies
between a reference equation, displayed code and worked result explicitly.

Independent synthetic fixtures must be labeled as such; their coefficient values
are not RC defaults. Source mappings live in `docs/reference/sources.md` at the
module root. Python notebooks are not executable dependencies of the Go tests.
