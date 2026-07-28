# PWA Read-State Persistence Design

## Problem

Editing a book in the web/PWA interface from `reading` to `read` is flaky. The PWA submits the complete editable field set on every save, including `body`. The server applies update fields one at a time by iterating a Go map, but `storage.Store.UpdateBook` rejects `body` even though the books table, model, create path, and PWA all support it.

The update therefore stops when it reaches `body`. Map iteration order is nondeterministic, so `status` may be persisted before the rejection or may remain `reading`. The changes endpoint still returns success and advances its version, after which the PWA can reload the incomplete server state. This explains why saving, refreshing, waiting for background sync, or changing another field produces inconsistent results.

## Decision

Extend `storage.Store.UpdateBook` to support the complete field set already submitted by the PWA. Specifically, accept `body`, apply it through the existing `applyMeta` path, and persist it to the existing `body` column.

Keep the PWA save flow and changes API shape unchanged. Do not add field diffing, retries, compatibility aliases, or a batch-update abstraction.

## Data Flow

1. The PWA opens a book whose status is `reading`.
2. The user selects `read` and saves.
3. The PWA stores the updated book locally and queues an update containing all editable fields.
4. The PWA posts the queued update to `/api/changes`.
5. The changes service applies every submitted field through `Store.UpdateBook`; `body` is now valid.
6. The server reports the update as applied and exposes the updated version.
7. A subsequent PWA sync or refresh reloads `status: "read"` rather than reverting to `reading`.

## Validation and Error Handling

Existing validation remains authoritative:

- `status` must be one of the model's canonical statuses.
- Ratings and numeric metadata retain their existing parsing and validation.
- `body` remains a free-form string, matching create/import behavior.
- Unknown update fields continue to return an error.

This fix does not alter the changes endpoint's broader partial-update or acknowledgement semantics.

## Testing

Add a regression test at the changes/API boundary using the same multi-field update shape emitted by the PWA. Start with a stored `reading` book, post an update containing `status: "read"` together with title, author, rating, tags, page count, publication year, ISBN, and `body`, then assert:

- the change summary reports one applied change and no skipped or errored changes;
- the stored status is `read`;
- the submitted body and other representative fields persist.

Add or extend storage-level coverage for direct `body` updates if the API regression does not already isolate that contract sufficiently.

Follow test-driven development: observe the regression fail because `body` is rejected, make the minimal storage change, then observe it pass.

## Verification

- Run focused tests for `internal/storage`, `internal/changes`, and `internal/api`.
- Run the full Go test suite.
- Generate and run the PWA with the API server.
- In a browser, edit a `reading` book to `read`, save, trigger sync/refresh, and confirm it remains `read`.
