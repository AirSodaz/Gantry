# ADR-038: Scheduled Trigger Time Semantics

**Status:** Accepted; completes the scheduling decision deferred by ADR-036 and
ADR-037.

**Date:** 2026-08-17

## Context

Owner-bound scheduled Triggers already share the normal Session, Run,
requester, approval, and occurrence-idempotency contracts. The remaining
ambiguity was how users express cadence, how local civil time maps to UTC, and
whether downtime creates catch-up Runs.

These rules must be deterministic across control-plane replicas and restarts.
They must also avoid a burst of stale work or duplicate Runs after downtime or
a daylight-saving transition.

## Decision

1. A scheduled Trigger uses a five-field POSIX-style cron expression with
   minute granularity: minute, hour, day of month, month, and day of week.
   Seconds, year fields, nicknames such as `@daily`, and executable command text
   are not accepted.
2. Every schedule requires one canonical IANA time-zone identifier. A numeric
   UTC offset is not a time zone. `UTC` is valid when local civil-time behavior
   is not required.
3. Gantry parses and validates the expression through a typed cron parser. The
   stored expression and time zone are data, never shell or evaluator input.
4. The only initial misfire policy is `skip`. On scheduler restart, Trigger
   re-enable, or recovery after an outage, uncommitted planned instants earlier
   than the recovery time do not create Runs and are not replayed in a burst.
5. A local time that does not exist because the clock moves forward is skipped.
   When the clock moves backward and a local time occurs twice, Gantry creates
   one occurrence at the first corresponding UTC instant.
6. Each scheduled configuration has a monotonically increasing
   `schedule_revision`. Changing the expression, time zone, fixed input,
   Deployment, or Session target increments it and recomputes the next planned
   instant without rewriting prior occurrences.
7. A due occurrence ID is derived from Trigger ID, schedule revision, and
   planned UTC instant. The occurrence, Session Message, queued Run, receipt,
   Audit evidence, next planned instant, and outbox entry commit atomically.
   Retries after that commit resume delivery but cannot create another Run.
8. A scheduled occurrence uses source tag `schedule`; the human Trigger owner
   remains the immutable Run requester and the fixed Service Principal remains
   execution-only.

## Consequences

- The schedule behaves consistently across replicas because one typed grammar,
  IANA time-zone data, and one overlap rule determine each planned UTC instant.
- Downtime may omit a scheduled Run. The UI and Audit history expose the next
  planned instant and one bounded recovery summary rather than enumerating
  skipped instants or implying catch-up.
- A busy bound Session can accumulate distinct future occurrences in Session
  order, but downtime itself does not manufacture a backlog.
- Updating a schedule cannot collide with an occurrence from an earlier
  configuration because the schedule revision participates in its identity.
- Catch-up, backfill, seconds-level schedules, and user-selectable misfire
  policies require a later explicit contract rather than an optional flag.

## Rejected Alternatives

- **Run every missed instant after recovery:** rejected because downtime could
  create an unsafe burst of stale work and approval requests.
- **Run both instants during a daylight-saving overlap:** rejected because a
  user-authored local schedule should not silently execute twice.
- **Store only a numeric UTC offset:** rejected because offsets do not preserve
  regional daylight-saving rules.
- **Accept arbitrary cron dialects or command suffixes:** rejected because their
  semantics differ and executable text would cross the scheduling boundary.
