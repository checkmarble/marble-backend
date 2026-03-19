Review authorization checks on resource IDs in the changed files.

For every resource ID received from user input (request body, query params, URL params) in the diff, verify:

1. The usecase fetches the resource by ID from the repository
2. It checks the resource belongs to the caller's organization using one of:
   - enforceSecurity.Read*(resource)
   - enforceSecurity.ReadOrganization(resource.OrganizationId)
   - Direct comparison: resource.OrganizationId != orgId
3. The error type is correct:
   - Public API (pubapi/): NotFoundError (404)
   - Internal API (api/): ForbiddenError (403) is acceptable

Common pitfalls to flag:
- enforceSecurity.Create*(callerOrgId) validates the caller's permission but does NOT check that IDs in the payload belong to the same org
- Repository Get*ById methods do NOT filter by org. They return any matching resource. The usecase must always verify org ownership after fetching.
- Validating the main returned resource but missing secondary IDs (e.g. inbox_id, tag_id, scenario_id in a creation payload)

Steps:
1. Run git diff on the current branch vs main to find changed files
2. Identify all handler and usecase files in the diff
3. For each changed usecase, find every resource ID parameter received from input
4. Check whether there is a fetch + org validation for each ID
5. Report findings as a checklist: resource ID, file, line, status (safe / missing check)
