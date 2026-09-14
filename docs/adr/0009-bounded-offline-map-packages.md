# ADR 0009: Start with one offline region and basemap using complete package updates

Date: 2026-09-14

Status: Accepted — scope and lifecycle boundary; provider and format undecided.

## Context

The operator endorsed a bounded first implementation alongside the explicit
prefetch workflow in [ADR 0008](0008-explicit-offline-map-preparation.md).
Maintaining every selectable map layer, arbitrary collections of regions or
incremental synchronization would substantially expand storage, download and
failure-handling requirements.

The current maps use raster tile sources. A local tile URL helper is available,
but it is not a downloader, durable map store or coverage guarantee. The fleet
and vehicle maps both need to consume the installed coverage.

## Decision

Start with **one saved operating region and one offline basemap**. Selecting a
new region deliberately replaces the installed region after successful
preparation. The offline basemap need not be the current online default; provider
selection must establish permitted download, offline use and applicable
attribution. Other online map layers do not implicitly become offline-capable.

Keep map data as durable local application data managed and served by the
backend, separate from the frontend build and disposable browser cache. Browser
cache clearing and application upgrades must not themselves delete installed
coverage. The frontend owns area selection, controls and visible status.

Use complete versioned package replacement for initial downloads and
operator-requested refreshes. Prepare a new package separately, verify it and
activate it only after successful completion. Keep the previous usable package
available on cancellation, interrupted download, validation failure or failed
activation. Plan for temporary disk space for both old and new packages; expose
insufficient space before discarding useful coverage. Do not implement tile-level
delta synchronization or automatic scheduled updates in the initial slice.

Record extent, detail limits, source, attribution, package version and download
time. Distinguish download time from source-data age: downloading today does not
establish that the underlying imagery was captured today. Include all resources
needed to render the chosen basemap offline; if vector tiles are selected this
includes styles, fonts/glyphs and sprites as applicable.

## Consequences and open choices

The scope contains the work to a local map service/store, preparation lifecycle,
shared map-source integration and a small operator surface. Telemetry acquisition
and vehicle state remain independent. A complete replacement may download more
than an incremental update, but gives a bounded recovery model.

Provider, raster versus vector, archive format, attribution, maximum area/detail,
storage budget, estimation accuracy and resumable-download support remain open.
PMTiles is a candidate, not an adopted architecture. Whole-state high-detail
imagery is not promised; measure a representative area before choosing bounds.
No provider is selected merely because it exposes public tile URLs.

The OpenStreetMap public raster tile service explicitly prohibits offline
prefetching. Esri documents offline use for eligible/export-enabled services;
that does not establish eligibility for the current URLs. Resolve source terms
before implementing acquisition. See the provider references below.

Multiple retained regions, multiple offline layers, automatic geographic
expansion and incremental/background refresh are deferred. This does not remove
existing online layer choices; their offline availability must be honest.

## Verification point

For the selected source and format, demonstrate an installed package surviving
browser cache clearing and application restart without network access. Test
both maps with the same package and verify complete resource locality. Exercise
refresh and area replacement with interruption, corrupt data, insufficient disk
space and restart at activation; the prior package must remain usable until a
verified replacement is active. Check source attribution and coverage metadata.

Record measured storage, download and temporary-space costs and remaining limits.
These checks are planned, not executed; no offline package is implemented by
accepting this ADR. Keep T-030's imagery-unavailable acceptance separate.

## References

- [OSM public raster tile usage policy](https://operations.osmfoundation.org/policies/tiles/)
- [Esri offline map eligibility](https://doc.arcgis.com/en/arcgis-online/manage-data/take-maps-offline.htm)
- [PMTiles regional extraction tools](https://docs.protomaps.com/pmtiles/cli)
