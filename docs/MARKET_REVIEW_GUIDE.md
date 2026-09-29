---
owner: @SivanCola
backup: @esengine
status: active
reviewed: 2026-09-29
---

# Review a community market package

## Purpose

Use this worksheet for one package version before calling it verified or recommending it. A submitted listing, a content digest, and a successful installation answer different questions. The review record MUST identify the exact version and environment behind each claim.

This guide records evidence for a reviewer; it does not grant approval or change a market listing. The market's existing review controls remain the publication path.

## Steps

1. Assign a reviewer and a backup for this review. If neither is available, leave the review pending. Record where the author accepts issue reports and whether the author still maintains the source. Do not promise an external response time without an owner who accepts it.
2. Fill in the identity and source fields below. Resolve a branch or tag to an immutable commit before testing. Retain the reviewed skill bytes, plugin commit and path, or MCP entry configuration as applicable.
3. Inspect the reviewed source and its declared dependencies for license terms, credentials, network access, executable code, and platform requirements. A repository's current license metadata alone does not establish the terms of the reviewed version or bundled dependencies.
4. Preview the exact source, then install it in an isolated test home. Compare the resolved source and capabilities with the review record. Record the install result and any trust warning. A preview or download alone does not establish that a task succeeds.
5. Run one minimal real task and one relevant failure case on every platform claimed as verified. Record the input, expected result, observed result, and evidence location. Keep private data and secrets out of the record.
6. Choose a disposition from the table below. Only a passing record for the same fixed package version, Studio version, and platform supports a verified claim. Recheck after a package update or a relevant Studio change. If the source changes during review, start a new version record.

Record the market digest separately from the reviewed source. Its coverage differs by package kind.

| Disposition | Meaning | Next action |
| --- | --- | --- |
| Pending | Evidence or an accountable reviewer is missing. | Keep the package out of verified recommendations; list the missing evidence. |
| Needs changes | A reproducible source, install, capability, or task check failed. | Give the author a minimal, redacted reproduction through the agreed issue channel. |
| Verified for stated scope | All checks passed for the recorded version, Studio build, and platforms. | Submit the record to the market reviewer before any recommendation change. |
| Suspend recommendation | A previously verified claim is no longer supported or its owner is unavailable. | Remove the recommendation through the existing review controls; assess package removal separately. |

Copy this record for each reviewed version:

```text
Package ID / kind / version:
Market listing URL / review record URL:
Reviewer / backup / review date:
Author issue channel / maintenance status:
Source repository / exact commit / package path:
Reviewed content or configuration digest / market digest and its coverage:
License at reviewed commit / dependency and asset licenses:
Studio version or commit / OS and architecture / required tools or services:
Declared capabilities / observed capabilities / trust warnings:
Preview result / install result / disable and remove result:
Minimal task input / expected result / observed result / evidence location:
Failure-case input / expected result / observed result / evidence location:
Unverified platforms or capabilities / open issues:
Disposition / reason / next review trigger:
```

When reporting a problem, include the fixed package source, Studio version, platform, minimal redacted input, expected result, observed result, and stable error code if one exists. Prepare a draft for the author or maintainer to inspect; send it only through a channel the reporter chooses.
