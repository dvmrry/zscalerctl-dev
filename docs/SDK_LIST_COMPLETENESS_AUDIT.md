# SDK List Completeness Audit

This audit records the production list helpers used by the resource readers at
the `6d5c903` main baseline with `zscaler-sdk-go/v3` `v3.8.38`. A function
named `GetAll` is not automatically complete or incomplete; its wire behavior
has to be checked.

The review resolved every SDK `GetAll*` call in `reader_zia.go`,
`reader_zpa.go`, `reader_ztw.go`, and `reader_zcc.go` to the vendored function
body, then checked the remaining list-returning handler calls (including ZTW
ZPA application segments) for the same pagination behavior. Zidentity was
reviewed separately because it uses the local `zidentityListAll` adapter rather
than an SDK `GetAll` helper.

## Classification

| Class | Meaning | Completeness posture |
|---|---|---|
| Local bounded paginator | zscalerctl requests each page and enforces a finite ceiling plus endpoint-specific progress checks | Preferred |
| SDK short-page paginator | SDK requests pages until a response contains fewer than the requested page size | Complete while the endpoint honors page and page-size parameters, but lacks a fail-closed page ceiling |
| SDK metadata paginator | SDK trusts response pagination metadata | Must reject missing, malformed, contradictory, or unreasonable metadata |
| Single-read list | SDK issues one GET and decodes a bare array | Correct only for an explicitly unpaginated/all-record endpoint or a documented finite ceiling |
| Singleton | One structured object, despite an SDK function name containing `GetAll` | Not a list-pagination concern |

## Disposition

| Product | Current disposition |
|---|---|
| ZIA | Every reader call that previously reached the SDK's unbounded `ReadAllPages` helper now uses a local bounded adapter. This includes methods whose names did not advertise pagination (PAC files, IPv6 prefixes, cloud-application policy views, CASB helpers, dedicated IP gateways, custom file types, sublocation lookup, and device lookup). Existing endpoint-specific behavior is retained: users keep the 10,000-record page size and service sort defaults; firewall filtering rules keep 5,000; location groups request member locations; and optional SDK filters that were unset remain unset. Eight list helpers issue a single read: SSL inspection rules, source IP groups, destination IP groups, time windows, NSS servers, sandbox rules, DC exclusions, and admin roles. No additional silent truncation was proven for those endpoints from source alone; they remain explicit live-validation targets. |
| ZPA | Twenty-four catalog resources previously shared the SDK's `GetAllPagesGeneric` implementation. That helper silently converted a missing or malformed `totalPages` value to zero, allowed every later page to rewrite the loop bound, and had no page ceiling. All 24 now use `getZPAAllPages`, which fixes the first-page count, rejects metadata drift, rejects repeated or empty declared pages, caps the walk at 1,000 pages, and discards partial results on any error. Client types and platforms are true singletons. CBI ZPA profiles and C2C IP ranges are documented raw-array list endpoints and remain single reads. |
| ZTW | All 19 paginated list resources use the local bounded `getZTWAllPages` adapter, including the non-`GetAll` ZPA application-segment helper. Admin roles remain an explicitly single-read list. No source-verifiable first-page truncation was found. |
| ZCC | All ten list resources use the local bounded `zccPaginate` adapter. Devices, admin roles, and the singleton-list fail-open policy were moved off the SDK's unbounded short-page helper in this batch. Company info is a singleton. |
| Zidentity | All three resources use the local offset paginator with a 1,000-page ceiling, response-offset progress validation, total-result handling, and repeated-page regression coverage. |

## ZPA Fail-Closed Contract

The ZPA adapter uses the API's existing 500-record page size and preserves the
SDK endpoint paths and microtenant query behavior. It returns no records when:

- `totalPages` is missing, null, malformed, negative, or above 1,000;
- a later response reports a different page count;
- a declared page is empty before collection completes;
- two consecutive nonempty pages are identical; or
- any page request or post-collection JMESPath filter fails.

This intentionally prefers a visible live-access failure over a successful but
possibly incomplete inventory.

## Regression Gate

`TestReadersAvoidVendoredUnboundedPagination` scans every production Go file in
the reader package and parses the vendored SDK packages it calls. The test
follows same-package helper calls and fails if any path reaches
`ReadAllPages*` or `GetAllPagesGeneric*`. This guards method names that obscure
pagination behavior and remains effective when the vendored SDK implementation
changes.

The bounded adapters also have multi-page transport tests, ceiling tests, and
partial-result failure tests. See
[SDK Pagination Validation](SDK_PAGINATION_VALIDATION.md) for the focused
automated and live checks.

## Remaining Work

Run focused live count checks for the explicitly single-read list endpoints and
the representative bounded resources. A successful HTTP 200 alone does not
prove completeness. Do not add speculative pagination to a single-read
endpoint unless its API contract or live behavior demonstrates a boundary.
