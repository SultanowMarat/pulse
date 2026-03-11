# pulse Testing Policy (Required)

This file defines mandatory testing rules for all future code in this repository.

## 1. Main Rule
- Any code change must include automated tests for the changed behavior.
- No feature/fix is considered complete without tests.

## 2. Scope Rules
- Backend (`go`): add/update `*_test.go` near changed package.
- Frontend (`web`): add/update unit tests (`*.test.ts` / `*.test.tsx`) for changed logic/components.
- Bug fixes: add a regression test that reproduces the bug and proves the fix.

## 3. Minimum Requirements Per Change
- At least one new or updated automated test for each changed behavior.
- Existing tests must remain green.
- Build must pass.

## 4. Local Commands (before merge)
- Backend: `go test ./...`
- Frontend tests: `cd web && npm run test`
- Frontend build: `cd web && npm run build`
- Full project check:
  - Windows PowerShell: `./scripts/test-all.ps1`
  - Linux/macOS: `./scripts/test-all.sh`

## 5. CI Enforcement
- GitHub Actions workflow `.github/workflows/ci-tests.yml` runs:
  - `go test ./...`
  - `web: npm ci && npm run test && npm run build`
- Merge is allowed only after green CI.

## 6. PR Checklist (must be true)
- [ ] I added/updated tests for all changed behavior.
- [ ] I added a regression test for each fixed bug.
- [ ] `go test ./...` passes.
- [ ] `cd web && npm run test` passes.
- [ ] `cd web && npm run build` passes.
