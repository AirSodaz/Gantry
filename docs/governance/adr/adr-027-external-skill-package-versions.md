# ADR-027: External Skill Package Versions

## Status

Accepted.

## Decision

Skills are imported from external package sources, such as an `npx skills`
marketplace or a Claude Code skills marketplace, or added manually as complete
package or local-directory artifacts. The package source owns the Skill's
declared version. Gantry does not create a Skill Draft, Revision, semantic
version, release channel, or deployment pointer.

An imported Skill Artifact records the source type and source reference, package
identity, the version declared by the package, normalized content, provenance,
validation state, and a content digest. A manual artifact uses the upload actor
and uploaded package digest as its source reference. Manual content is not
edited in Gantry; a change is imported as another complete artifact. The declared version is
display metadata; if a package declares no version, the UI shows `未声明` and
Gantry does not synthesize one. The source reference and digest identify the
exact artifact for authorization and reproducibility. If multiple artifacts
declare the same package version but have different content, Gantry keeps them
distinct and exposes the difference.

Multiple artifacts for one Skill may coexist in a workspace. Test Agent
bindings may select different artifacts so package updates can be compared
without changing Production. An Agent Revision pins one exact artifact. A
later import or external package update never mutates an existing Agent
Revision or run manifest.

Gantry may validate, activate, deprecate, quarantine, or retire an imported
artifact, but these are catalog and runtime-governance states, not package
version operations.

## Consequences

The Skills UI stays an import and inspection surface: it supports marketplace,
direct-locator, complete-package upload, and local-directory intake, and shows package-declared
versions, source provenance, digests, activation state, and Agent usage. It does
not expose a second version editor or release workflow. Supporting multiple
artifacts preserves safe testing while keeping package ownership and version
semantics with the upstream marketplace.
