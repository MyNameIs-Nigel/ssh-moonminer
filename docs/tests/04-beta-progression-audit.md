# Beta 2.0.0 progression audit

Scope: gameplay/08 only. Quests, dedicated hunting, engineering, ship naming,
stations, and cosmetics remain outside this change.

## Test-first evidence and affected tests

The first executable acceptance failure was `TestFrontierContentAndDepartureBoundary`:
the embedded catalog had **2 systems / 4 hulls**, rather than 4 / 7. New API tests
initially failed compilation, then ran against the implemented APIs. Additional
behavioral regressions were observed before their fixes:

- Tribute removed physical cargo without reducing its frontier provenance.
- Redline had no stability clock.
- The old recovery advance could not launch a local run in Eridani, Kepler, or Redline.
- A free local Skiff was refused when another Skiff was parked remotely.
- A malformed legacy inventory was silently discarded during migration.
- The jump overlay's mouse cancel target overlapped the confirmation action.

The existing installed-route-key removal tests were retired: their replacement
is the table over every gate, both directions, and Classes uncertified/E/D/C.
Inventory tests now reject retired devices; the v6 migration test asserts the
rating grant while retaining its missile conversion assertions. Existing local
permit, hull, cargo, shield, combat, and module-condition tests remain in force.
The shipyard row test now covers seven models and the Lantern's second Internal
slot. Responsive tests drop the retired drive-row anchor, retain descriptions,
and locate chart hitboxes from rendered text rather than obsolete row numbers.

Coverage includes:

- Every certification requirement one short, wrong certification system,
  insufficient credits, and rejection without mutation.
- Seeded jump replay; each drift result; first-crossing misalignment exclusion;
  Scanner S drift reduction; the exact 20% warning boundary; no-drift projection
  equality and final result equality after drift.
- Bare-Skiff gate access, the two/three S-tank Mule boundary, ferry pricing,
  active-hull rejection, all unrated paths, and personally visited destinations.
- Frontier survival including disconnect autopilot, cargo mining origin across
  a return jump and sale, tribute deductions, and death/credential persistence.
- Home markets, Lantern modules, Vesper shield/buyback restrictions, arrival-only
  repair, and preserving a remote Skiff when issuing a local rescue hull.
- v6/v7 upgrades, a real v7 JSON blob round-tripped through SQLite, no duplicate
  refund on reloading v8, and deep isolation of every added reference field.
- Actor-owned snapshots and reconnect after a committed jump.
- Jump screen semantics and golden frames at 80×24 and 144×48, cancellation
  hitboxes, timer invalidation when skipping, and reduced-motion bypass.

## Resolved implementation details

The doc's recommended beta choices are used: inventory remains account-wide;
frontier builders use the `frontier` brand; misalignment has low weight only on
the Kepler–Redline gate and is excluded until a successful first crossing.
Class E is certified in Sol.

Jump confirmation shows guaranteed transit costs **before drift**, explicitly
labels them, and lists the possible drift losses. Its numbers exactly match a
no-drift jump. The arrival card uses the committed result, including drift.
Showing an exact random result in the preflight checklist would reveal the
outcome before the cinematic; the distinction resolves the original acceptance
wording. The actor commits the complete jump once when confirmed, before any
animation. A dropped connection or skipped countdown cannot double-charge or
leave the save halfway between docks.

The first legacy drive buys Class E; only additional copies refund at the old
base price times sell percentage, regardless of the retired device's grade.
Legacy cargo with an unknown mining origin pays its full credit value but does
not invent frontier qualification. Old run logs seed only reconstructable
per-destination survival, kill, and Legendary counters.

A rescue Skiff is issued locally without overwriting an existing remote Skiff.
The old hull receives an internal stable hangar key (`skiff@<system>` plus a
collision suffix if needed); its model, fitting, hull, fuel, and location are
preserved. Every hull remains visible and ferryable. These identifiers do not
implement user-facing ship names.

Balance gaps exposed by tests required three authored adjustments:

- Eris has no capacity gate, so an unfitted recovery Skiff can mine locally.
  Its existing combat pressure remains; Nadir and Sable retain tank gates.
- Frontier docks provide a 900cr salvage advance (a base refuel), while Sol
  retains 300cr. Eligibility uses the local route's fuel need.
- Kepler offers Wayfarer (32 fuel) and Longreach (65 fuel / 180 capacity).
  Redline offers Ember (36 fuel) and Rupture (75 fuel / 200 capacity).
  Their stability declines during mining at 0.2% / 0.4% per second; at zero,
  the field collapses and destroys a ship still drilling. The mining HUD
  shows stability and the remaining time. Vesper repairs 8% on belt-to-dock
  arrival; repeated Dock calls and crew transfers do not grant repairs.

A pre-existing stats error also counted a Legendary after extracting zero ore.
Both lifetime and frontier Legendary counts now require actual extraction.

## Verification

Required release gates are build, vet, ordinary tests, and the full race suite
(including real SSH loopback tests), plus the local MinIO restore drills because
the save payload schema changed. Run them on the final worktree before pushing
`beta`. This change does not deploy production or alter release infrastructure.

Completed on the final beta worktree:

- `go build ./...`, `go vet ./...`, `go test ./...` — passed.
- `CGO_ENABLED=1 go build -race ./...`, `go vet -race ./...`, and
  `go test -race ./...` — passed, including SSH listener tests.
- Save decoder fuzzing — 573,845 executions without race instrumentation;
  another 60,676 executions under `-race`, both passed.
- Rebuilt the real Linux/amd64 image with its Go 1.26.4 toolchain, then passed
  `kill-drill.sh`, `restore-to-scratch.sh`, `no-credentials-check.sh`, and
  `run.sh`. The kill drill restored a genuine save with clean integrity and
  decode checks and a measured **2-second RPO**. Scratch restore independently
  decoded the replicated save; network-isolated dev mode served without S3.

Historical dock-sale records also preserve known system visits during migration,
while their cargo sale values remain uncredited to a mining origin that cannot
be recovered. This has a dedicated regression test.
