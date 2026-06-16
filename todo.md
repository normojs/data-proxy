# data-proxy MCP / Bridge TODO

## P1 - Release preflight and dirty ownership

- [x] Classify follow-up Fusion benchmark request-option and early-exit changes.
  - Acceptance: tracked dirty changes in `tools/fusion-benchmark.mjs` and `tools/fusion-benchmark/config.json` are either committed as project-owned benchmark functionality or explicitly reverted only if proven accidental.
  - Done: kept the changes as project-owned Fusion benchmark support for per-model upstream request options and exact-majority early exit, documented `modelOptions` / `earlyExit` in the benchmark README, and added Qwen request-option plus Fusion preset early-exit examples in the shared config.
- [x] Classify follow-up Fusion benchmark report metric changes.
  - Acceptance: tracked dirty changes in `tools/fusion-benchmark.mjs` are either committed as project-owned report functionality or explicitly reverted only if proven accidental.
  - Done: kept the changes as project-owned Fusion benchmark reporting support for early-exit rate and panel/judge/final stage latency metrics, and documented the extra report fields in the benchmark README.
- [x] Classify current dirty files before release checks.
  - Acceptance: identify which files are project-owned source/docs/config examples and which are local-only runtime artifacts.
  - Done: `tools/fusion-benchmark.mjs`, `tools/fusion-benchmark/README.md`, `tools/fusion-benchmark/config.json`, `tools/fusion-benchmark/.env.example`, and `tools/fusion-benchmark/data/*.example.jsonl` are classified as project-owned Fusion benchmark tooling; `tools/fusion-benchmark/.env.local`, `tools/fusion-benchmark/runs/`, `tools/fusion-benchmark/reports/`, and `tools/fusion-benchmark/secrets/` are local-only and ignored.
- [x] Validate and commit dirty ownership handling.
  - Acceptance: benchmark tool syntax check, secret scan, whitespace check, and git status confirm only intended files are staged/committed.
  - Done: `node --check tools/fusion-benchmark.mjs`, `node tools/fusion-benchmark.mjs help`, sensitive pattern scan over staged benchmark files, and `git diff --check` passed; staged files exclude `.env.local`, `runs/`, and `reports/`.
- [x] Run release preflight after the ownership commit.
  - Acceptance: default deployment preflight passes or any failure is documented with exact command and next fix.
  - Done: `make deployment-preflight` passed `go test ./...`, `make build-all-frontends`, production Compose config, and dev Compose config before being interrupted at the Docker daemon gate because `docker version >/dev/null` hung on Server response.
  - Done: follow-up checks passed for `gtimeout 20 docker compose config`, `gtimeout 20 docker compose -f docker-compose.dev.yml config`, `gtimeout 10 docker buildx version`, and `git diff --check`; `gtimeout 10 docker version` still timed out after printing Docker Client info only, so release image validation is blocked on local Docker daemon responsiveness.
- [x] Run non-Docker release regression after the Docker blocker recheck.
  - Acceptance: code-level checks that do not require Docker pass while the Docker daemon gate remains explicitly blocked.
  - Done: `gtimeout 15 docker version` and `gtimeout 15 docker info` still timed out while reading Server information, and Docker's Unix socket still returned `Docker Desktop is unable to start`.
  - Done: `go test ./...`, `cd web/default && bun run typecheck`, locale JSON parsing, `make build-all-frontends`, `make mcp-regression`, and `git diff --check -- . ':!tools/fusion-benchmark.mjs'` passed. Fusion benchmark dirty files were intentionally excluded from this current-session scope.
- [x] Restore Docker daemon responsiveness before tagging a release image.
  - Acceptance: `docker version`, `docker info`, default `make deployment-preflight`, and optional `DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight` complete without hanging.
  - Current status: Docker Desktop remains a local environment blocker. `ps` shows long-running `com.docker.backend`, `com.docker.build`, and `docker-sandbox daemon start` processes; `gtimeout 10 docker version` prints client info then times out before server info; `curl --unix-socket /Users/fushilu/.docker/run/docker.sock http://localhost/_ping` returns `Docker Desktop is unable to start`.
  - Rechecked 2026-06-16: `gtimeout 15 docker version` and `gtimeout 15 docker info` still time out while reading Server information; Docker's Unix socket still returns `Docker Desktop is unable to start`.
  - Rechecked after `docker desktop start`: Docker Desktop reports it is already running, but `docker version` / `docker info` still time out before Server details and the Unix socket still returns `Docker Desktop is unable to start`.
  - Done: Docker daemon later recovered; `gtimeout 15 docker version` and `gtimeout 15 docker info` both returned Server details.
  - Done: `make deployment-preflight` passed. `DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight` initially hit a local frontend `SIGKILL` before Docker build, then a direct `docker build --target builder2 -t new-api:preflight-builder .` passed, and the full optional preflight passed on retry with cached Docker layers.

## P1 - Current non-Docker audit follow-up

- [x] Align runtime config JSON helper usage.
  - Acceptance: runtime-config load/save paths use the project JSON wrappers instead of direct standard-library JSON calls, while `common/json.go` remains the wrapper boundary.
  - Done: `common/runtime_config.go` now uses `common.Unmarshal` / `common.MarshalIndent`; the package-level JSON scan reports only `common/json.go` wrapper implementation calls.
- [x] Align active product/design context with the Data Proxy brand.
  - Acceptance: active AI/design context files describe Data Proxy, not New API, while upstream attribution docs remain unchanged.
  - Done: `PRODUCT.md` and `DESIGN.md` now describe Data Proxy as the active product/design system, and no longer retain obsolete v1/v2 reversibility guidance after the legacy UI deletion.
- [x] Validate and commit the non-Docker audit follow-up.
  - Acceptance: targeted Go tests, runtime JSON scan, brand context scan, whitespace checks, and git commit complete without staging Fusion exploration files.
  - Done: `go test ./common ./controller -run 'TestPostSetupRuntimeConfig|TestRuntimeConfig|TestLoadRuntimeConfig|TestSaveRuntimeConfig'`, common runtime JSON scan, `rg -n "New API|new-api" PRODUCT.md DESIGN.md`, and scoped `git diff --check` passed.

## P1 - Runtime brand residual audit

- [x] Clean residual runtime JSON conversion calls in OAuth, middleware, and settings.
  - Acceptance: OAuth providers, request-conversion middleware, exchange-rate fetching, and simple setting parsers use `common.Marshal`, `common.Unmarshal`, or `common.DecodeJson` instead of direct runtime `encoding/json` marshal/decode calls.
  - Acceptance: DTO custom JSON methods, `json.RawMessage` type boundaries, and test helper JSON calls remain intentionally out of scope.
  - Done: OAuth token/profile decoding, Jimeng/Kling request conversion, Turnstile/exchange-rate response decoding, and simple settings JSON parsing now route runtime conversion through project JSON helpers while preserving DTO/custom JSON boundaries.
- [x] Refresh stale non-MCP audit classifications after residual JSON cleanup.
  - Acceptance: `docs/non-mcp-backlog-audit.md` no longer lists already-completed non-JSON request parsing behavior as remaining technical debt and records the residual JSON cleanup batch.
  - Done: `docs/non-mcp-backlog-audit.md` now records the residual JSON cleanup as completed and reclassifies `common/gin.go` unknown content-type parsing as an explicit no-op contract rather than remaining implementation debt.
- [x] Validate and commit residual JSON cleanup.
  - Acceptance: targeted Go packages, residual runtime JSON scan, `git diff --check`, and `todo.md` are updated before commit.
  - Done: `go test ./oauth ./middleware ./service ./setting/...`, targeted residual runtime JSON scan over OAuth/middleware/exchange-rate/settings non-test files, and `git diff --check` passed before commit.
- [x] Remove actionable runtime brand leftovers from panic, notification, payment, and locale surfaces.
  - Acceptance: runtime panic responses, Gotify notification headers, payment reference/default object names, and user-switchable locale values use Data Proxy where the text belongs to this product.
  - Acceptance: Go module paths, upstream attribution URLs, and preserved upstream repository labels remain unchanged.
  - Done: replaced panic response copy/error type, Gotify User-Agent, Stripe reference prefix, Waffo Pancake buyer/default names, and non-attribution locale values; residual scan only reports module/upstream attribution boundaries.

- [x] Audit visible runtime references to upstream New API branding.
  - Acceptance: distinguish Go module/import paths and upstream attribution from user-visible Data Proxy runtime copy.
  - Done: `rg` shows many `github.com/QuantumNous/new-api` module paths and upstream docs/links that should remain as source attribution; actionable runtime findings were limited to channel helper copy, stale locale fallback values, and the dashboard update-check User-Agent.
- [x] Replace actionable runtime brand leftovers.
  - Acceptance: Chinese and English runtime strings no longer display New API for Data Proxy-owned placeholders, channel helper copy, or default fallback text.
  - Done: updated channel credential/warning copy to Data Proxy/OpenAI-compatible wording, changed the update-check User-Agent to `data-proxy-dashboard`, and aligned stale Chinese/English locale fallback values for console placeholder, sender placeholder, product name, and welcome text.
- [x] Decide whether to maintain upstream README/electron packaging as attribution or create Data Proxy-specific release docs.
  - Acceptance: repository-level docs and optional desktop packaging have an explicit branding policy so future audits do not mix source attribution with product runtime copy.
  - Done: added `docs/branding-and-release-policy.md`, kept top-level README/electron packaging classified as upstream attribution material for now, and made Data Proxy-specific release/operator docs the active release path. `docs/deployment-readiness.md` now links the policy for release checks.

## P1 - MCP market mock example polish

- [x] Add realistic mock examples next to parameter templates.
  - Acceptance: each MCP market tool detail keeps the existing input schema and parameter template actions, and adds a separate Mock Example action immediately after Parameter Template.
  - Done: added a schema-aware mock JSON-RPC example dialog that fills common fields such as paths, commands, sessions, timezones, URLs, package names, limits, booleans, arrays, and nested objects with realistic sample values.
- [x] Validate and commit this batch.
  - Acceptance: locale JSON parsing, MCP i18n missing-key scan, frontend typecheck/build check, whitespace checks, and git commit complete.
  - Done: locale JSON parse, MCP component missing-key scan, `cd web/default && bun run typecheck`, `cd web/default && bun run build:check`, and `git diff --check` passed.

## P1 - API key base URL and MCP Chinese copy polish

- [x] Keep the API key Base URL copy action next to the address text.
  - Acceptance: the API address and copy button remain visually grouped on desktop and mobile; long addresses truncate before the button instead of pushing it to the far edge.
  - Done: constrained the Base URL code field width and kept the copy button in the same inline flex group.
- [x] Localize MCP market and subpage static copy.
  - Acceptance: MCP Market, Overview, MCP Tools, Proxy, Bridge, OpenAPI object, audit, billing, reconciliation, empty-state, toast, and action copy have Chinese translations.
  - Done: filled the missing MCP component translation keys, added dynamic MCP status/source/category/schema labels, and added an explicit “工具介绍” label in the market detail panel.
- [x] Localize built-in MCP tool introductions and parameter descriptions.
  - Acceptance: seeded built-in MCP tools display Chinese names, descriptions, and input schema property descriptions in the market and tool detail surfaces.
  - Done: updated the built-in MCP catalog seed definitions; existing deployments can refresh these records through startup seeding or the MCP seed action.
- [x] Validate and commit this batch.
  - Acceptance: locale JSON parsing, MCP i18n missing-key scan, frontend typecheck/build check, targeted Go tests, whitespace checks, and git commit complete.
  - Done: locale JSON parse, MCP component/dynamic missing-key scan, `cd web/default && bun run typecheck`, `cd web/default && bun run build:check`, `go test ./pkg/mcp/catalog ./model -run 'TestSeed|TestMCP|TestBuiltin|TestOpenAPI|TestProxy'`, and `git diff --check` passed.

## P1 - H 站 OAuth and dashboard scope split

- [x] Add an H 站 OAuth preset for the dc.hhhl.cc bridge.
  - Acceptance: operators can choose the H 站 preset in Custom OAuth, use `read:profile`, and see the exact Data Proxy callback URL pattern to register in H 站.
  - Done: added the `dc-hhhl` preset with bridge OAuth endpoints, `read:profile`, stable field mappings, and a read-only Data Proxy callback URL hint that distinguishes direct OAuth callbacks from bridge callbacks.
- [x] Rename the Chinese Playground menu label.
  - Acceptance: the `/playground` route keeps its URL, but Chinese UI uses a clearer product label instead of “游乐场”.
  - Done: Chinese UI now labels Playground as “模型调试” while keeping the `/playground` route unchanged.
- [x] Split personal and site-wide dashboard scope.
  - Acceptance: General / Dashboard always queries the current account, including admin users; a separate Admin / Site Dashboard entry above Channels queries site-wide model usage.
  - Done: dashboard usage APIs now take an explicit `self` / `site` scope; General / Dashboard uses current-account data, Admin / Site Dashboard points to `/dashboard/site-models`, and site-wide username filtering plus performance health only appear in the site-wide view.
- [x] Validate and commit this batch.
  - Acceptance: frontend typecheck, locale JSON parse, whitespace checks, and targeted tests pass or any skipped checks are recorded.
  - Done: locale JSON parse, targeted Prettier check, `cd web/default && bun run typecheck`, `cd web/default && bun run build:check`, and `git diff --check` passed.

## P1 - Transparent Data Proxy brand assets

- [x] Remove the white background from generated Data Proxy brand assets.
  - Acceptance: default logo and favicon assets preserve transparent backgrounds across the web UI, browser tab icon, and generated icon sizes.
  - Done: regenerated `web/default/public/logo.png` as PNG32 with transparent background and regenerated `web/default/public/favicon.ico` with transparent 256/128/64/48/32/16 sizes from `/Users/fushilu/Pictures/dataproxy.png`.
- [x] Validate and redeploy transparent brand assets locally.
  - Acceptance: generated assets have alpha channels, the frontend build succeeds, and the local Docker container serves the refreshed assets.
  - Done: `magick web/default/public/logo.png ...` confirmed transparent alpha on the generated logo, `make build-all-frontends` passed, `docker compose build data-proxy && docker compose up -d data-proxy` passed, the `data-proxy` container is healthy, and `curl` checks confirmed served `/logo.png` plus `/favicon.ico` have transparent corner pixels.

## P1 - Exchange rate controls and home page redesign

- [x] Make automatic exchange-rate refresh configurable.
  - Acceptance: operators can configure the refresh interval, the default is explicit, and values below the minimum are rejected before saving.
  - Done: added `exchange_rate_auto_update_interval_minutes`, defaulted it to 720 minutes, enforced a 60-minute minimum in backend validation, exposed it through status/options, and covered interval normalization with service tests.
- [x] Fetch the current exchange rate when automatic refresh is enabled.
  - Acceptance: enabling automatic refresh from the pricing settings page triggers an immediate fetch after the setting is saved, while the background worker continues to use the configured interval.
  - Done: pricing settings now call the exchange-rate fetch path immediately when auto-update is newly enabled; the background job reads the current interval each cycle.
- [x] Keep manual exchange-rate fetching available.
  - Acceptance: operators can still fetch the current rate on demand and use it to fill the manual display-rate field.
  - Done: kept the manual `Fetch current rate` control in the pricing section and aligned helper copy for manual and automatic update flows.
- [x] Redesign the public home page around Data Proxy's gateway role.
  - Acceptance: the default home page feels like a production operations control plane rather than a generic AI landing page, and keeps the newer UI as the only runtime surface.
  - Done: rebuilt the hero, stats, feature, lifecycle, and CTA sections around governed model/MCP/Bridge traffic, provider routing, quota/billing, OpenAPI binary objects, and operational review.
- [x] Validate and deploy the updated local container.
  - Acceptance: backend tests, frontend typecheck, locale parsing, Docker rebuild/restart, and browser verification pass.
  - Done: `go test ./controller ./service ./setting/operation_setting`, locale JSON parse, `cd web/default && bun run typecheck`, `git diff --check`, `docker compose build data-proxy`, and `docker compose up -d data-proxy` passed. Browser verification at `http://localhost:3000/` confirms the new Data Proxy home page is served and the old hero copy is absent.

## P1 - Currency, announcements, and MCP guidance

- [x] Add an optional free USD exchange-rate refresh path for pricing display.
  - Acceptance: operators can fetch the current USD-to-payment-currency rate on demand, enable automatic background refresh, and keep manual rates available when the free provider is unavailable.
  - Done: added the Frankfurter free public provider integration, root-only `/api/option/exchange-rate/fetch`, 12-hour master-node auto-update task, status fields, pricing UI controls, and Chinese/English copy for manual/automatic update states.
- [x] Add required-reading announcements with explicit read controls.
  - Acceptance: announcement editors can mark a system announcement as required reading; logged-in users can mark one or all announcements as read; required announcements are not silently marked read just by opening the notification center.
  - Done: added persisted per-user announcement read state, notification read APIs, required-reading validation, management UI checkbox/table column, notification popover badges, single-read and mark-all-read actions, and localStorage fallback for anonymous/offline paths.
- [x] Replace color-only MCP categories and explain MCP pages.
  - Acceptance: MCP category display uses icons plus text instead of category color alone; every MCP subpage has concise purpose copy; the MCP horizontal section navigation remains tidy at narrow widths.
  - Done: added stable lucide icon category badges, reused them in the MCP market list/detail, added registry-backed MCP section descriptions, and changed MCP tabs to horizontal scrolling instead of wrapped rows.
- [x] Validate and commit this batch.
  - Acceptance: Go formatting/tests, frontend typecheck, locale JSON parsing, and whitespace checks pass; commit excludes unrelated local `.gitignore` and benchmark files.
  - Done: `go test ./model ./controller ./router ./service`, locale JSON parse, `cd web/default && bun run typecheck`, and `git diff --check` passed. The commit excludes unrelated local `.gitignore` and benchmark files.

## P1 - First-run dependency setup wizard

- [x] Move first-install database and Redis configuration into the setup wizard.
  - Acceptance: default Docker startup does not require `SQL_DSN` / `REDIS_CONN_STRING`; the setup API reports dependency state; uninitialized systems can test and save database/Redis runtime config before creating the first administrator account.
  - Done: added local `runtime-config.json` loading before DB/Redis initialization, setup status dependency fields, `POST /api/setup/runtime-config`, DB/Redis connection tests, and setup wizard controls for SQLite/MySQL/PostgreSQL plus optional Redis.
- [x] Keep `.env` as an advanced operations override instead of the recommended first-install path.
  - Acceptance: explicit environment variables still win, but Docker Compose and primary README guidance direct users to configure existing dependencies in the first-run wizard.
  - Done: updated `.env.example`, `docker-compose.yml`, `docker-compose.dev.yml`, `README.md`, and `README.zh_CN.md`; default Compose services now start only `data-proxy`, with PostgreSQL/Redis available only under the optional `local-deps` profile.
- [x] Make bundled and existing local dependencies explicit in first-run setup.
  - Acceptance: first startup runs only the Data Proxy container; bundled PostgreSQL/Redis are opt-in and do not conflict with services already running on the host; setup UI gives operators clear presets for bundled Compose services and existing host services.
  - Done: kept bundled PostgreSQL/Redis behind the `local-deps` profile, removed fixed container names, avoided publishing bundled dependency ports to the host, aligned dev Redis auth with production defaults, and added setup wizard buttons for bundled PostgreSQL/Redis plus host PostgreSQL/MySQL/Redis connection strings. Validation: `docker compose -f docker-compose.dev.yml config --services`, `docker compose config --services`, `docker compose -f docker-compose.dev.yml --profile local-deps config --services`, `docker compose --profile local-deps config --services`, locale JSON parse, `cd web/default && bun run typecheck`, `go test ./common ./model ./controller ./router`, and `git diff --check` passed.
- [x] Clarify bundled-vs-local dependency choices in the setup wizard.
  - Acceptance: first-run dependency setup explains when bundled PostgreSQL/Redis run, which Compose hostnames to use, that bundled services do not publish host ports, and when to use `host.docker.internal` for existing local services.
  - Done: replaced the single helper line with concise PostgreSQL/MySQL/Redis decision notes, including explicit no-host-port guarantees for bundled PostgreSQL `5432` and Redis `6379`. Validation: `cd web/default && bun run typecheck`, locale JSON parse, `docker compose config --services`, `docker compose --profile local-deps config --services`, and `git diff --check` passed.
- [x] Clarify host selection for existing local dependencies.
  - Acceptance: setup copy no longer implies that `host.docker.internal` is always correct; it distinguishes Docker-to-host access, same-machine direct access, and remote network access.
  - Done: updated PostgreSQL/MySQL/Redis setup notes to say `host.docker.internal` is only for Data Proxy running in Docker while the dependency runs on this Mac, `127.0.0.1` is for direct same-machine runtime, and network IP/domain is for dependencies on another machine. Validation: `cd web/default && bun run typecheck`, locale JSON parse, and `git diff --check` passed.
- [x] Prevent initialization from writing the first administrator into the temporary database after runtime config is saved.
  - Acceptance: after saving runtime config, the setup UI requires a Data Proxy restart before moving past the dependency step or submitting final initialization.
  - Done: setup status exposes `runtime_config_restart_required`; the setup wizard blocks next/submit and shows a restart-required alert until the server restarts with the saved config.
- [x] Validate setup wizard dependency configuration.
  - Acceptance: backend tests, frontend type/build checks, locale parsing, Docker Compose default-service checks, and whitespace checks pass.
  - Done: `go test ./common ./model ./controller ./router`, `cd web/default && bun run typecheck`, locale JSON parse, `docker compose -f docker-compose.dev.yml config --services`, `docker compose config --services`, `make build-all-frontends`, and `git diff --check` passed. Added controller regression tests for setup runtime config rejection/success paths.

## P1 - Data Proxy runtime branding

- [x] Replace default runtime product naming with Data Proxy.
  - Acceptance: backend defaults, startup/help output, browser title/meta, setup/settings defaults, and layout fallbacks no longer show New API as the product name.
  - Done: backend `SystemName`, startup/help text, browser title/meta, default frontend constants, layout fallbacks, setup/settings defaults, and product placeholders now use `Data Proxy` while upstream compatibility references remain explicit.
- [x] Replace default web logo/favicon assets from `/Users/fushilu/Pictures/dataproxy.png`.
  - Acceptance: `web/default/public/logo.png` and `web/default/public/favicon.ico` are regenerated from the provided image and used by the default frontend.
  - Done: regenerated `web/default/public/logo.png` as 512x512 PNG and `web/default/public/favicon.ico` with 256/128/64/48/32/16 sizes from `/Users/fushilu/Pictures/dataproxy.png`.
- [x] Validate branding changes and commit.
  - Acceptance: targeted Go tests, frontend typecheck/build, and whitespace checks pass; this TODO batch records exact validation commands and the commit is created.
  - Done: `go test ./common ./controller ./setting/system_setting`, `node -e "const fs=require('fs'); for (const f of fs.readdirSync('web/default/src/i18n/locales').filter(f=>f.endsWith('.json'))) JSON.parse(fs.readFileSync('web/default/src/i18n/locales/'+f,'utf8')); console.log('locale json ok')"`, `cd web/default && bun run typecheck`, `make build-all-frontends`, and `git diff --check` passed.

## P1 - Default-only new UI runtime

- [x] Make the backend runtime serve only the new UI by default.
  - Acceptance: server-side theme defaults to `default`, existing `classic` values are normalized away, and web routing serves `web/default` assets/index only.
  - Done: backend theme state now initializes and normalizes to `default`, Web routing serves only `web/default` assets/index, legacy theme-aware static serving was removed, and smoke/dev embed helpers only prepare `web/default/dist`.
- [x] Remove the legacy frontend switch from the new UI settings surface.
  - Acceptance: system settings no longer show a selectable legacy frontend option and form validation accepts only the new UI value.
  - Done: system settings now show a read-only new-frontend status instead of a classic/default selector, and validation accepts only the `default` frontend value.
- [x] Align build/deployment defaults with new UI only.
  - Acceptance: default frontend build and deployment preflight build only `web/default`; legacy UI sources are not part of the normal runtime/deploy path.
  - Done: `web` workspace, Dockerfile, Dockerfile.dev, `make build-all-frontends`, `make dev-web`, and deployment preflight now target the new frontend only.
- [x] Validate new UI default behavior and update docs.
  - Acceptance: targeted Go tests, frontend type/build checks, and `git diff --check` pass; TODO/docs record that classic is no longer a runtime option.
  - Done: `go test ./common ./router ./setting/system_setting ./controller`, `go test ./...`, `cd web && bun install --frozen-lockfile`, `cd web/default && bun run typecheck`, `make build-all-frontends`, `make -n deployment-preflight`, and `git diff --check` passed. `docs/deployment-readiness.md` and `docs/ui-v2-long-term-plan.md` now record that only the newer `web/default` UI is served by default.

## P2 - Runtime monitor resilience

- [x] Prevent optional pprof CPU monitor errors from crashing the process.
  - Acceptance: `common.Monitor` logs transient CPU sampling errors and continues instead of panicking in a background goroutine.
  - Done: `common.Monitor` now logs `cpu monitor sample failed` and continues the background loop when CPU sampling fails.
- [x] Refresh panic audit classification for pprof monitor.
  - Acceptance: backlog audit no longer classifies `common/pprof.go` as startup fail-fast behavior.
  - Done: `docs/non-mcp-backlog-audit.md` now records pprof monitor errors as runtime-log-and-continue behavior.
- [x] Validate monitor package and scan results.
  - Acceptance: targeted common package tests pass and panic scan shows the pprof runtime panic has been removed.
  - Done: `go test ./common`, `rg -n "panic\\(" common/pprof.go docs/non-mcp-backlog-audit.md`, and `git diff --check` passed.

## P2 - Intentional unsupported route guardrails

- [x] Cover OpenAI-compatible intentionally unsupported relay response shape.
  - Acceptance: `controller.RelayNotImplemented` returns HTTP 501 with stable OpenAI-style error code/type/message.
  - Done: added controller coverage for HTTP 501 plus `api_not_implemented` / `new_api_error` / `API not implemented` response fields.
- [x] Cover intentionally unsupported relay route registration.
  - Acceptance: `/v1/files`, `/v1/fine-tunes`, `/v1/images/variations`, and delete-model routes remain registered to the explicit not-implemented handler rather than drifting to 404 or a generic relay path.
  - Done: added router registration coverage for files, fine-tunes, image variations, and model delete paths to ensure they stay wired to `controller.RelayNotImplemented`.
- [x] Validate route guardrails and update audit notes.
  - Acceptance: targeted controller/router tests pass and the backlog audit records that explicit 501 routes are now regression-covered.
  - Done: `go test ./controller ./router -run 'TestRelayNotImplemented|TestRelayNotImplementedRoutes'` and `git diff --check` passed; `docs/non-mcp-backlog-audit.md` now records the explicit route guardrail coverage.

## P2 - Residual TODO comment cleanup

- [x] Replace ambiguous production TODO comments with explicit maintenance notes.
  - Acceptance: remaining code TODO scan findings are limited to documentation/history, not active implementation ambiguity.
  - Acceptance: behavior stays unchanged.
  - Done: replaced the legacy MySQL `model_mapping` migration TODO with an upgrade-floor maintenance note and replaced the model pricing TODO with an explicit pricing guardrail.
- [x] Refresh backlog audit classification after cleanup.
  - Acceptance: `docs/non-mcp-backlog-audit.md` records the cleanup and still distinguishes intentional unsupported boundaries from real backlog.
  - Done: refreshed the non-MCP backlog audit to record the maintenance-note cleanup while preserving intentional unsupported/backlog classifications.
- [x] Validate targeted packages and TODO scan.
  - Acceptance: targeted Go tests pass for touched packages and `rg` confirms no active code `TODO:` comments remain outside documentation/TODO tracking files.
  - Done: `go test ./model ./setting/ratio_setting`, `rg -n "TODO:" --glob '*.go' --glob '!web/**/dist/**' --glob '!vendor/**' .`, and `git diff --check` passed.

## P1 - Deployment preflight ergonomics

- [x] Add a repeatable deployment preflight Make target.
  - Acceptance: one Make target runs backend tests, both production frontend builds, production/dev compose config validation, Docker toolchain checks, and whitespace checks.
  - Acceptance: the full Docker image build is opt-in so local release checks do not hang indefinitely on external base image metadata pulls.
  - Done: added `make deployment-preflight`, which runs `go test ./...`, both frontend production builds, production/dev Compose config validation, Docker engine/buildx checks, and `git diff --check`; full Docker image build is opt-in through `DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1`.
- [x] Document deployment readiness gates and caveats.
  - Acceptance: docs explain the default preflight, the optional Docker image build, generated frontend artifact handling, and the known Docker Hub metadata caveat.
  - Done: added `docs/deployment-readiness.md` with the default gate, optional Docker image build command, generated `dist` handling, and current Docker Hub metadata caveat.
- [x] Run and record deployment preflight ergonomics validation.
  - Acceptance: Makefile syntax/command expansion is verified and the non-network parts of the new gate are exercised or mapped to the commands already run in this preflight batch.
  - Done: `make -n deployment-preflight`, `git diff --check`, and default `make deployment-preflight` passed. The default gate intentionally skipped the full Docker image build and left `web/default/dist` as ignored generated artifact.

## P0 - Deployment readiness preflight

- [x] Run full backend test and build preflight.
  - Acceptance: `go test ./...` passes or any intentional skips/failures are documented with exact commands and reasons.
  - Acceptance: the backend binary can be built with the current frontend assets expectation satisfied or explicitly covered by Docker/frontend build checks.
  - Done: `go test ./...` passed. Backend binary build is covered by the Docker multi-stage build and will use frontend assets produced by the frontend/Docker checks below.
- [x] Run production frontend build preflight for the active UI.
  - Acceptance: `make build-all-frontends` passes for `web/default`.
  - Acceptance: generated artifacts remain uncommitted unless the repository already tracks them.
  - Done: `make build-all-frontends` passed for the default UI. Generated `web/default/dist` is ignored by git and was not committed.
- [x] Validate Docker deployment configuration.
  - Acceptance: `docker compose config` passes for production and dev compose files.
  - Acceptance: Docker build prerequisites are verified without committing local build artifacts.
  - Done: `docker compose config` and `docker compose -f docker-compose.dev.yml config` passed after removing the obsolete top-level Compose `version` field from `docker-compose.yml`; Docker engine `29.2.0` and buildx `v0.31.1-desktop.1` are available. `docker build --target builder2 -t new-api:preflight-builder .` was attempted twice but canceled after stalling on Docker Hub base image metadata pulls for `golang:1.26.1-alpine` / `oven/bun:1`; no local build artifacts were committed.
- [x] Record deployment readiness result.
  - Acceptance: `todo.md` records exact commands run, pass/fail result, and remaining deployment caveats.
  - Acceptance: final worktree is clean after any generated artifacts are cleaned or intentionally ignored.
  - Done: deployment readiness preflight passed for backend tests (`go test ./...`), production frontend builds (`make build-all-frontends`), production compose config (`docker compose config`), dev compose config (`docker compose -f docker-compose.dev.yml config`), and whitespace checks (`git diff --check`). Remaining caveat: a full local Docker image build still depends on Docker Hub base image metadata/network availability; retry `docker build --target builder2 -t new-api:preflight-builder .` before tagging a release image.

## P1 - Fresh audit follow-up batch

- [x] Clarify Claude extended-thinking sampling compatibility.
  - Acceptance: duplicated temporary handling for Claude thinking sampling parameters is replaced with explicit helper logic or documented guardrails.
  - Acceptance: tests cover non-Opus extended thinking and Opus 4.7/4.8 adaptive thinking across the OpenAI-to-Claude conversion path.
  - Done: added explicit `dto.ClaudeRequest` helpers for adaptive thinking effort, sampling clearing, default extended-thinking temperature, and OpenAI conversion sampling compatibility; replaced duplicated temporary handling in native Claude and OpenAI-to-Claude paths. Tests now cover Opus 4.8 adaptive high-effort and non-Opus enabled-budget compatibility. Validation: `go test ./dto ./relay ./relay/channel/...`.
- [x] Classify non-JSON request model extraction behavior.
  - Acceptance: `common.UnmarshalBodyReusable` behavior for unknown content types is covered by tests or documented as intentional no-op behavior.
  - Acceptance: distributor model extraction remains predictable for JSON, form, multipart, and unknown content types.
  - Done: replaced the vague non-JSON TODO with an explicit no-op contract for unknown content types; added `common` tests for JSON, form, multipart, body reset, repeated reads, and unknown content type no-op; added distributor model extraction tests for JSON, form, multipart, and unknown content type. Validation: `go test ./common ./middleware`.
- [x] Clarify Jimeng task result code normalization.
  - Acceptance: Jimeng task result parsing has tests for success, provider failure, queue, and done statuses.
  - Acceptance: provider codes are either normalized through a named helper or documented as intentionally preserved upstream codes.
  - Done: added named Jimeng success-code helpers; success code `10000` maps to local `0`, provider failures preserve upstream code, and failure status can no longer be overwritten by `data.status=done`. Tests cover done, queue, provider failure, failure-with-done, and invalid JSON. Validation: `go test ./relay/channel/task/jimeng ./relay/channel/task/...`.
- [x] Refresh release regression after fresh audit batch.
  - Acceptance: selected Go package checks, `git diff --check`, and the relevant MCP regression target pass after the fresh audit fixes.
  - Acceptance: `todo.md` records the verification result and any intentionally skipped external dependency gates.
  - Done: `go test ./common ./middleware ./dto ./relay ./relay/channel/... ./relay/channel/task/...`, `git diff --check`, and `make mcp-regression` passed after the fresh audit batch. External MySQL/PostgreSQL migration gates were not rerun in this step because this batch did not touch migrations and the MCP regression target was the relevant release guard.

## P0 - Current development plan

- [x] Add a repeatable Docker-backed PostgreSQL migration gate.
  - Acceptance: local developers can run a documented command or Make target that starts/uses project-owned PostgreSQL and executes `make mcp-migration-postgres` with a known test DSN.
  - Acceptance: the gate does not depend on unrelated host containers or production databases.
  - Done: added `docker-compose.migration.yml` and `make mcp-migration-postgres-docker`; the target starts disposable PostgreSQL on `127.0.0.1:15432`, runs `make mcp-migration-postgres`, and cleans up by default.
- [x] Add a repeatable Docker-backed MySQL migration gate.
  - Acceptance: local developers can run a documented command or Make target that starts/uses project-owned MySQL and executes `make mcp-migration-mysql` with a known test DSN.
  - Acceptance: the gate handles existing host port conflicts such as a global `mysql8` container on port `3306`.
  - Done: added disposable MySQL to `docker-compose.migration.yml` and `make mcp-migration-mysql-docker`; the target uses `127.0.0.1:13306` by default, runs `make mcp-migration-mysql`, and cleans up by default.
- [x] Run and record external database migration gates.
  - Acceptance: PostgreSQL and MySQL migration smoke results are recorded in `todo.md` after the Docker-backed gates run.
  - Acceptance: failures are fixed or explicitly documented with reproduction commands.
  - Done: `make mcp-migration-docker` passed, running PostgreSQL on `127.0.0.1:15432` and MySQL on `127.0.0.1:13306` through disposable Docker services.
- [x] Harden external migration gate documentation and cleanup.
  - Acceptance: docs explain startup, DSN, reset, cleanup, and how the gates relate to `make mcp-regression`.
  - Acceptance: no secrets or host-specific credentials are committed beyond disposable local test defaults.
  - Done: documented Docker-backed startup, default DSNs, port overrides, debug retention, cleanup, and the relationship between `make mcp-migration-docker` and `make mcp-regression`.
- [x] Audit non-MCP backlog and pick the next backend batch.
  - Acceptance: remaining `TODO` / `unsupported` / `not implemented` findings outside the completed MCP scope are classified as product backlog, intentional unsupported behavior, or bug-fix candidates.
  - Done: classified non-MCP findings in `docs/non-mcp-backlog-audit.md`; selected asynchronous channel balance refresh, conservative image URL sensitive checks, and SSRF-safe Gemini/Veo HTTP image input support as the next backend batch.

## P1 - Selected next backend batch

- [x] Make admin-triggered all-channel balance refresh asynchronous.
  - Acceptance: `UpdateAllChannelsBalance` returns promptly, prevents overlapping runs, and exposes enough status/log context to know whether a refresh is already running or has started.
  - Acceptance: the existing single-channel balance query remains synchronous.
  - Done: admin all-channel balance refresh now starts a background job, returns `started` and `refresh` state, rejects overlapping refreshes with the current running snapshot, and the automatic refresher shares the same lock.
- [x] Add conservative sensitive checks for image URL payloads.
  - Acceptance: `CheckSensitiveMessages` scans text-bearing image URL fields already present in request payloads without fetching remote images.
  - Acceptance: tests cover text-only, image-only, mixed content, and no-sensitive-word paths.
  - Done: `CheckSensitiveMessages` now scans text, image URL strings, `MessageImageUrl` fields, and nested textual image metadata without fetching remote content; service tests cover text, image URL string/object, direct metadata, and clean mixed content.
- [x] Add SSRF-safe HTTP image input support for Gemini/Veo task images.
  - Acceptance: HTTP image URLs are downloaded only through safe request helpers with size and MIME limits, then converted to existing base64 image input.
  - Acceptance: tests cover allowed URLs, rejected hosts, oversized content, unsupported MIME, data URI, and raw base64 behavior.
  - Done: Gemini/Veo task image parsing now downloads HTTP(S) inputs through `service.DoDownloadRequest`, preserves SSRF redirect checks, enforces 20 MB and supported image MIME limits, and keeps data URI/raw base64 support covered by tests.

## P2 - Realtime compatibility hardening

- [x] Make Realtime WebSocket subprotocol compatibility explicit.
  - Acceptance: the local WebSocket upgrader supports stable Realtime protocol names expected by known clients.
  - Acceptance: credential-bearing `openai-insecure-api-key.*` values are not selected or echoed as WebSocket subprotocols.
  - Acceptance: tests cover accepted protocol negotiation and unsupported protocol behavior without requiring upstream connectivity.
  - Done: Realtime WebSocket negotiation now explicitly supports `realtime` and `openai-beta.realtime-v1`, keeps `openai-insecure-api-key.*` out of the selectable subprotocol list, and has handshake-level tests for accepted and unsupported protocols.

## P2 - Task lifecycle hygiene

- [x] Make task polling context and resource handling explicit.
  - Acceptance: long-running task pollers no longer use `context.TODO()` for production loops.
  - Acceptance: Midjourney polling HTTP requests cancel their timeout context and close response bodies on every response/error path.
  - Acceptance: touched JSON serialization paths use the project `common` wrappers and tests cover the extracted polling request helper.
  - Done: task polling loops now use explicit background contexts; Midjourney polling uses an extracted timeout-aware request helper with deferred cancel/body close, project JSON wrappers, and tests for success, status errors, parse errors, and missing base URL.

## P2 - Veo advanced image inputs

- [x] Add Veo last-frame and reference-image request support.
  - Acceptance: Gemini and Vertex Veo request builders can map metadata `lastFrame` / `last_frame` into the request instance when a primary image is present.
  - Acceptance: metadata `referenceImages` / `reference_images` supports up to 3 safely parsed image inputs with optional `referenceType` / `reference_type`, defaults to `asset`, and is limited to Veo 3.1 models.
  - Acceptance: reference-image requests default duration to 8 seconds when not provided and reject non-8 second durations before calling upstream.
  - Acceptance: tests cover payload shape, validation failures, and Gemini/Vertex shared behavior.
  - Done: Gemini/Vertex Veo builders now share advanced image parsing for `lastFrame` and `referenceImages`, reuse safe image parsing for base64/data URI/HTTP inputs, enforce Veo 3.1 and reference duration constraints, and cover helper plus both request builders in tests.

## P2 - Channel balance capability clarity

- [x] Make Azure channel balance behavior explicit.
  - Acceptance: Azure channels are explicitly classified as unsupported for API-key balance queries instead of being left as a generic TODO.
  - Acceptance: all-channel background balance refresh skips Azure before invoking provider balance requests.
  - Acceptance: single-channel Azure balance queries return a typed unsupported error and tests cover the helper/error behavior.
  - Done: Azure balance querying now returns a typed unsupported error, all-channel refresh skips Azure before provider requests, and controller tests cover the capability helper and unsupported error.

## P2 - UI v2 pilot

- [x] Activate UI v2 design context.
  - Acceptance: `PRODUCT.md`, `DESIGN.md`, a UI v2 design brief, and Impeccable live/design config exist before implementation.
  - Acceptance: the design direction evolves `web/default` as the shadcn-based pilot home.
  - Done: added product/design context, `.impeccable` live/design files, and `docs/ui-v2-design-brief.md` for the active pilot.
- [x] Add a UI version switcher and persisted preference.
  - Acceptance: operators can switch between current UI and v2 pilot without losing the existing routes.
  - Acceptance: the choice is stored locally first and can be promoted to backend preference later.
  - Done: added a localStorage-backed UI version store and a profile-menu switch that preserves the current UI while enabling the v2 pilot preference.
- [x] Add the v2 authenticated pilot shell.
  - Acceptance: the v2 shell is mounted under a low-risk authenticated route and uses existing auth, router, i18n, and shadcn primitives.
  - Acceptance: the shell includes navigation, page header, breadcrumbs or context, density rules, and mobile-safe layout.
  - Done: added the admin-guarded `/ui-lab/*` shell, profile switch navigation, route tree entries, compact v2 navigation, breadcrumb context, and a placeholder MCP pilot route ready for data wiring.
- [x] Build the MCP operations v2 pilot surface.
  - Acceptance: the pilot reuses existing MCP APIs/query keys and covers overview, health/risk state, recent activity, and navigation into existing detailed sections.
  - Acceptance: loading, empty, partial, error, and permission states are represented.
  - Done: `/ui-lab/mcp` now uses the existing `getMCPSummary` API and `mcpQueryKeys.summary`, with KPI tiles, risk strip, operations trends, review queue, top tools, recent errors, detailed drill-ins, and explicit loading/empty/partial/error/admin states.
- [x] Validate UI v2 pilot and decide rollout status.
  - Acceptance: route smoke, typecheck/build, and browser screenshots pass.
  - Acceptance: TODO records whether v2 remains a pilot or is ready for a broader rollout.
  - Done: `bun run smoke:mcp-routes`, `bun run smoke:mcp-trends`, `bun run typecheck`, and `bun run build` passed on 2026-06-13.
  - Done: desktop (1440x1000) and mobile (390x844) browser screenshot validation passed against a mock-auth local preview with `/api/user/self`, `/api/status`, and `/api/mcp/summary`; no console errors were captured.
  - Rollout: keep UI v2 as a pilot behind `/ui-lab/*` and the persisted version switcher; it is deploy-safe because the current UI remains available, but broader rollout should wait for real backend/auth staging validation.

## P0 - Release readiness

- [x] Run unified MCP regression after final architecture cleanup.
  - Acceptance: `make mcp-regression` passes after the latest MCP/OpenAPI/Proxy cleanup commits.
  - Done: `make mcp-regression` passed, covering OpenAPI, Proxy, Bridge lightweight checks, Dashboard smoke, and TypeScript build.
- [x] Run SQLite MCP migration smoke.
  - Acceptance: `make mcp-migration-sqlite` passes against a temporary SQLite database.
  - Done: `make mcp-migration-sqlite` passed against the temporary SQLite migration smoke, including repeated `InitDB` / `InitLogDB` startup on the same database.
- [x] Run real Bridge daemon concurrency smoke.
  - Acceptance: `make mcp-bridge-smoke` passes with local daemon read/write/edit/glob/proxy coverage.
  - Done: `make mcp-bridge-smoke` passed with `MCP_BRIDGE_SMOKE_CONCURRENCY=4`, `MCP_BRIDGE_SMOKE_ITERATIONS=1`, `MCP_BRIDGE_SMOKE_TIMEOUT=120000`, temp SQLite WAL/busy-timeout pragmas, and single SQLite connection.
- [x] Fix SQLite repeated migration for Bridge daemon smoke.
  - Acceptance: SQLite startup migration can run twice against the same database without `invalid DDL, unbalanced brackets`.
  - Acceptance: `make mcp-migration-sqlite` covers the repeated-start regression.
  - Done: SQLite migration now normalizes legacy `decimal(p,s)` table DDL to `numeric` before repeated AutoMigrate parsing.
- [x] Harden Bridge daemon smoke against performance monitor guard.
  - Acceptance: smoke setup avoids CPU/memory/disk monitor 503 responses caused by local test machine load.
  - Done: Bridge smoke prepare raises CPU, memory, and disk monitor thresholds to 100 for local release-gate runs.
- [x] Align Bridge daemon smoke with server-side write policy.
  - Acceptance: write/edit smoke calls explicitly enable server-side Bridge policy allowlist before expecting success.
  - Done: smoke policy setup now sets `allowed_tools: ["*"]`, `allow_write: true`, and `mcp_allowed_targets: ["*"]` for the writable daemon client.
- [x] Record external database migration gate status.
  - Acceptance: MySQL/PostgreSQL migration commands are either executed with DSNs or documented as DSN-gated.
  - Done: MySQL/PostgreSQL migration gates are DSN-gated on this machine because `MCP_MIGRATION_MYSQL_DSN` and `MCP_MIGRATION_POSTGRES_DSN` are unset; run `make mcp-migration-mysql MCP_MIGRATION_MYSQL_DSN='...'` and `make mcp-migration-postgres MCP_MIGRATION_POSTGRES_DSN='...'` when external test databases are available.
- [x] Complete final release hygiene audit.
  - Acceptance: `git diff --check` passes, worktree is clean, and remaining TODO/unsupported scan findings are classified.
  - Done: `git status --short --branch` showed a clean worktree, `git diff --check` passed, `make mcp-regression` passed, and the remaining scan hits are limited to `todo.md` documentation plus smoke-script fail-fast helper `panic` calls.

## P1 - MCP Proxy HTTP client architecture cleanup

- [x] Align MCP Proxy HTTP JSON-RPC serialization with project wrappers.
  - Acceptance: `pkg/mcp/proxy/http_client.go` no longer calls `json.Marshal` or `json.Unmarshal` directly.
  - Acceptance: `encoding/json` remains only for JSON-RPC `json.RawMessage` DTO types.
  - Acceptance: MCP Proxy HTTP/SSE/Streamable tests still pass.
  - Done: MCP Proxy HTTP client marshal/unmarshal paths now use `common.Marshal/common.Unmarshal`; proxy package tests and `make mcp-proxy-check` passed.

## P1 - MCP controller architecture cleanup

- [x] Remove unnecessary direct JSON dependency from MCP controller.
  - Acceptance: `controller/mcp.go` no longer imports `encoding/json`.
  - Acceptance: MCP initialize and invalid request controller tests still pass.
  - Done: MCP controller now passes `req.ID` directly to `common.JsonRawMessageToString`; MCP controller tests passed.

## P1 - OpenAPI parser architecture cleanup

- [x] Align OpenAPI parser JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/openapi/parser.go` no longer imports `encoding/json`.
  - Acceptance: OpenAPI parser tests still cover refs, schema merges, form/multipart, and binary request bodies.
  - Done: OpenAPI parser now uses `common.Unmarshal/common.Marshal` for JSON parsing and schema cloning; parser and OpenAPI package tests passed.

## P1 - OpenAPI binary storage architecture cleanup

- [x] Align OpenAPI binary object metadata JSON conversion with project wrappers.
  - Acceptance: local and S3 binary object stores no longer import `encoding/json` for metadata serialization.
  - Acceptance: binary object save/load/cleanup tests still pass for local and S3-compatible stores.
  - Done: local and S3 metadata save/load/cleanup now use `common.Marshal/common.Unmarshal`; OpenAPI package binary object tests passed.

## P1 - MCP executor architecture cleanup

- [x] Align built-in MCP executor JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/executor/builtin.go` no longer imports `encoding/json`.
  - Acceptance: built-in executor server time and JSON pretty tests still pass.
  - Done: built-in executor server time and JSON pretty now use `common.Marshal/common.UnmarshalJsonStr/common.MarshalIndent`; executor tests passed.

## P1 - OpenAPI executor architecture cleanup

- [x] Align OpenAPI MCP executor JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/executor/openapi.go` no longer imports `encoding/json`.
  - Acceptance: OpenAPI executor request/response formatting tests still pass.
  - Done: OpenAPI executor now uses `common.Marshal/common.Unmarshal/common.MarshalIndent`; MCP OpenAPI regression passed.

## P1 - MCP Proxy architecture cleanup

- [x] Align MCP Proxy Bridge client JSON conversion with project wrappers.
  - Acceptance: `pkg/mcp/proxy/bridge_client.go` no longer imports `encoding/json`.
  - Acceptance: Bridge proxy list/call/test behavior remains covered by existing tests.
  - Done: Bridge client result/object helpers now use `common.Marshal/common.Unmarshal`; MCP Proxy regression passed.

## P1 - Billing architecture cleanup

- [x] Replace subscription pre-consume string matching with sentinel errors.
  - Acceptance: model subscription pre-consume errors support `errors.Is`.
  - Acceptance: billing session maps no-active/insufficient subscription errors to insufficient quota without string matching.
  - Done: model now exposes subscription pre-consume sentinel errors; billing session uses `errors.Is` and tests cover model/service behavior.
- [x] Make subscription post-consume settlement idempotent by request.
  - Acceptance: subscription settlement/finalize retries for the same `request_id` do not apply the final delta more than once.
  - Acceptance: conflicting replay deltas for an already-settled request return a typed error instead of silently mutating subscription usage.
  - Done: subscription pre-consume records now persist `post_consumed_delta` and `settled_at`; subscription funding settlement uses a request-scoped idempotent model path, zero-delta settlements are marked final, and service tests cover replay plus conflict behavior.

## P1 - Relay regression cleanup

- [x] Fix Claude relay OpenAI file-content conversion.
  - Acceptance: unsupported OpenAI `file` content is skipped instead of being sent as an image.
  - Acceptance: PDF files become Claude `document` blocks and text files become Claude `text` blocks.
  - Acceptance: `go test ./relay/channel/claude` and `go test ./relay/channel/...` pass.
  - Done: Claude relay now infers OpenAI file mime type from filename when needed, accepts map payload `filename`, emits PDF/document and text blocks explicitly, and ignores unsupported files.
- [x] Cover StreamScannerHandler per-call StreamStatus reset semantics.
  - Acceptance: pre-existing stream status state from a reused relay info object is not carried into the next stream scan.
  - Done: added a regression test asserting StreamScannerHandler replaces any pre-initialized StreamStatus and starts the new scan with a clean error count.
- [x] Align Coze relay JSON conversion with project wrappers.
  - Acceptance: Coze response decode and OpenAI response encode paths use `common.Unmarshal` / `common.Marshal` instead of direct `encoding/json` conversion calls.
  - Done: Coze adaptor and relay handlers now keep `encoding/json` only for `json.RawMessage` type references while routing runtime JSON conversion through project wrappers.
- [x] Align relay model mapping JSON parsing with project wrappers.
  - Acceptance: model mapping helper no longer imports `encoding/json` only to parse the channel mapping string.
  - Done: `ModelMappedHelper` now uses `common.Unmarshal` for model mapping parsing while preserving the existing cycle-detection behavior.
- [x] Stabilize StreamScannerHandler tests that mutate global streaming settings.
  - Acceptance: parallel stream scanner tests cannot restore `constant.StreamingTimeout` or ping interval settings while another stream scanner test is still running.
  - Done: stream scanner tests now serialize global streaming setting mutations through a package-level test lock while keeping the existing behavior coverage.
- [x] Align simple rerank provider JSON conversions with project wrappers.
  - Acceptance: Ali and SiliconFlow rerank response decode/encode paths no longer call `encoding/json` directly.
  - Done: Ali and SiliconFlow rerank handlers now use `common.Unmarshal` / `common.Marshal` for runtime JSON conversion.
- [x] Align Palm and Jimeng provider JSON conversions with project wrappers.
  - Acceptance: Palm response conversion and Jimeng image/signature helper JSON conversion no longer call `encoding/json` directly.
  - Done: Palm stream/non-stream response paths and Jimeng image response, payload hash, and image extra-field parsing now use project JSON wrappers.
- [x] Align Cloudflare and Cohere provider JSON conversions with project wrappers.
  - Acceptance: Cloudflare and Cohere stream/non-stream response decode/encode paths no longer call `encoding/json` directly.
  - Done: Cloudflare completions/STT and Cohere chat/rerank handlers now use `common.Unmarshal` / `common.Marshal`.
- [x] Align Baidu and Dify provider JSON conversions with project wrappers.
  - Acceptance: Baidu response/token decode and Dify upload/chat response conversion paths use project JSON wrappers.
  - Done: Baidu chat/embedding/token handling and Dify upload/user/stream/blocking response handling now use `common.DecodeJson`, `common.Unmarshal`, or `common.Marshal` as appropriate.
- [x] Align MokaAI, Tencent, Xunfei, and Zhipu provider JSON conversions with project wrappers.
  - Acceptance: simple response/signature JSON conversion paths in these providers no longer call `encoding/json` directly.
  - Done: MokaAI embedding, Tencent chat/signature payload, Xunfei websocket response, and Zhipu stream/non-stream response paths now use project JSON wrappers.
- [x] Align Minimax and Volcengine TTS JSON conversions with project wrappers.
  - Acceptance: TTS metadata request construction and provider response parsing no longer call `encoding/json` directly in Minimax and Volcengine paths.
  - Done: Minimax and Volcengine TTS request/response conversion now uses `common.Unmarshal` / `common.Marshal` while preserving existing request payload shapes.
- [x] Align Vertex, Replicate, OpenAI, and AWS focused JSON conversions with project wrappers.
  - Acceptance: single-purpose provider JSON conversion points use project wrappers while keeping `json.RawMessage` type references intact where needed.
  - Done: Vertex extra-body/token decode, Replicate output format, OpenAI THINKING parsing, AWS anthropic beta/Nova response handling now use `common` JSON helpers.
- [x] Align Claude relay runtime JSON conversions with project wrappers.
  - Acceptance: Claude tool-call argument parsing and OpenAI-format response encoding use project JSON wrappers while preserving `json.RawMessage` type references.
  - Done: Claude relay now uses `common.Unmarshal` / `common.Marshal` for runtime conversion paths covered by the provider package tests.
- [x] Align Gemini relay runtime JSON conversions with project wrappers.
  - Acceptance: Gemini function response/tool call parsing and OpenAI image response encoding use project JSON wrappers while preserving `json.RawMessage` thought-signature fields.
  - Done: Gemini relay now uses `common.Unmarshal` / `common.Marshal` for runtime conversion paths covered by provider package tests.
- [x] Align Ollama relay runtime JSON conversions with project wrappers.
  - Acceptance: Ollama request schema/tool argument parsing, version decode, stream chunk parsing, thinking decode, and tool-call argument encoding use project JSON wrappers while preserving `json.RawMessage` DTO fields.
  - Done: Ollama relay and stream handlers now use `common.Unmarshal` / `common.Marshal` for runtime conversion paths.
- [x] Align OAuth and external status controller JSON conversions with project wrappers.
  - Acceptance: GitHub, Discord, OIDC, LinuxDO, WeChat, and Uptime Kuma controller JSON encode/decode paths no longer call `encoding/json` directly.
  - Done: those controllers now use `common.Marshal` / `common.DecodeJson` for request payloads and HTTP JSON response decoding.
- [x] Align channel balance controller JSON response parsing with project wrappers.
  - Acceptance: provider balance response parsing in `controller/channel-billing.go` no longer calls `encoding/json` directly.
  - Done: OpenAI-compatible, AIProxy, API2GPT, AIGC2D, SiliconFlow, DeepSeek, OpenRouter, and Moonshot balance parsing now uses `common.Unmarshal`.
- [x] Align user controller runtime JSON conversions with project wrappers.
  - Acceptance: user login/register/update/admin request decode and sidebar/default-config JSON conversion use project wrappers while preserving `json.Number` type handling.
  - Done: `controller/user.go` now uses `common.DecodeJson`, `common.Marshal`, and `common.Unmarshal` for runtime conversions.
- [x] Align remaining small controller JSON conversions with project wrappers.
  - Acceptance: deployment, misc, Creem top-up, and model metadata controller JSON conversions use project wrappers.
  - Done: `controller/deployment.go`, `controller/misc.go`, `controller/topup_creem.go`, and `controller/model_meta.go` now route runtime JSON conversion through `common` helpers.
- [x] Finish controller runtime JSON conversion cleanup.
  - Acceptance: controller runtime JSON conversion scan has no direct `json.Marshal`, `json.Unmarshal`, `json.NewEncoder`, or `json.NewDecoder` calls outside intentional `json.RawMessage` type references.
  - Done: console migration, model sync, and channel controller JSON paths now use `common` helpers; `controller` package tests pass.
- [x] Align service notification/session JSON conversions with project wrappers.
  - Acceptance: user notification, webhook, worker download payload, Codex OAuth claim decode, and passkey session JSON conversion paths use project wrappers.
  - Done: those service paths now use `common.Marshal` / `common.Unmarshal` while preserving `json.RawMessage` DTO fields where required.
- [x] Align Midjourney service JSON conversions with project wrappers.
  - Acceptance: Midjourney request body rewrite and response parsing no longer call `encoding/json` directly.
  - Done: `service/midjourney.go` now uses `common.DecodeJson`, `common.Marshal`, and `common.Unmarshal` for runtime JSON conversion.
- [x] Align service conversion JSON helpers with project wrappers.
  - Acceptance: Claude/OpenRouter and Gemini conversion helpers in `service/convert.go` no longer call `encoding/json` directly for runtime marshal/unmarshal paths.
  - Done: `service/convert.go` now uses `common.Marshal` and `common.Unmarshal` for reasoning metadata, tool arguments, and generic helper serialization.
- [x] Align model runtime JSON conversions with project wrappers.
  - Acceptance: model-layer setting, pricing endpoint, passkey transport, channel metadata, and prefill JSON serialization paths use project wrappers while preserving `json.RawMessage` type references.
  - Done: `model/pricing.go`, `model/user.go`, `model/passkey.go`, `model/channel.go`, and `model/prefill_group.go` now route runtime conversion through `common` helpers where applicable.
- [x] Align common utility JSON conversions with project wrappers.
  - Acceptance: common utility helpers use the package JSON wrapper functions instead of direct standard-library calls, while `common/json.go` remains the wrapper implementation boundary.
  - Done: `common/utils.go`, `common/topup-ratio.go`, and `common/str.go` now call `Marshal` / `Unmarshal` directly.
- [x] Align cache codec and Midjourney proxy JSON conversions with project wrappers.
  - Acceptance: Redis JSON codec and Midjourney proxy task/response conversion paths no longer call `encoding/json` directly.
  - Done: `pkg/cachex/codec.go` and `relay/mjproxy_handler.go` now use `common.Marshal` / `common.Unmarshal` for runtime JSON conversion.
- [x] Align io.net client JSON conversions with project wrappers.
  - Acceptance: io.net request encoding, response/error decoding, flexible-time normalization, and query array encoding no longer call `encoding/json` directly.
  - Done: `pkg/ionet` now uses `common.Marshal` / `common.Unmarshal` across client, deployment, hardware, container, and JSON utility paths.
- [x] Finish cross-module runtime JSON conversion cleanup.
  - Acceptance: runtime `json.Marshal`, `json.Unmarshal`, `json.NewEncoder`, and `json.NewDecoder` scan across controller/service/model/pkg/relay/common is clean outside the `common/json.go` wrapper boundary and non-runtime comments.
  - Done: global scan now reports only `common/json.go` wrapper implementation calls and one explanatory comment in `relay/channel/task/taskcommon/helpers.go`.

## P1 - Audit remediation

- [x] Fix Bridge reconnect/offline race so an old session close cannot mark a replaced live client offline.
  - Acceptance: closing an old session while a replacement session is online keeps `bridge_clients.status=online`.
  - Acceptance: normal last-session close still marks the client offline.
- [x] Make MCP Overview Bridge online trends accurate beyond the fixed 10k session cap.
  - Acceptance: recent buckets are not undercounted when many sessions overlap the window.
  - Acceptance: tests cover overflow-like session counts without relying on production-size fixtures.
- [x] Make MCP Review Queue large-installation behavior explicit and harder to miss.
  - Acceptance: queue summaries expose capped scan/visible counts or overflow metadata.
  - Acceptance: dashboard can distinguish “no issues” from “scan capped”.
  - Done: Review Queue responses now expose `visible_count`, `max_items`, `truncated`, and per-source `scan_limits`; MCP Overview shows visible/total counts and capped scan state.
- [x] Replace reachable provider adaptor `panic("implement me")` stubs with stable unsupported errors.
  - Acceptance: unsupported relay modes return errors instead of panicking.
  - Done: 11 provider `ConvertClaudeRequest` stubs now return stable `not implemented` errors; regression test covers no-panic behavior.
- [x] Align Bridge controller JSON decoding with project JSON wrapper rules.
  - Acceptance: Bridge controller no longer calls `encoding/json` marshal/unmarshal directly.
  - Done: `controller/bridge.go` now uses `common.Marshal/common.Unmarshal`; controller tests cover bridge result/error payload decoding.
- [x] Add shared unsupported adaptor error semantics.
  - Acceptance: provider adaptor stubs can return a typed unsupported-feature error with provider and feature context while preserving existing user-facing `not implemented` wording.
  - Acceptance: tests can assert typed unsupported errors without brittle string-only checks.
  - Done: added `channel.UnsupportedFeatureError` / `NewUnsupportedFeatureError` and typed unsupported adaptor tests.
- [x] Migrate generic provider adaptor stubs in small batches.
  - Acceptance: each batch removes generic `errors.New("not implemented")` returns from touched provider adaptors without implementing unsupported capabilities.
  - Acceptance: provider/channel tests pass after each batch.
  - Done first batch: Ali `ConvertGeminiRequest` / `ConvertAudioRequest` and XAI `ConvertGeminiRequest` now return typed unsupported-feature errors.
  - Done second batch: Baidu, Cloudflare, Cohere, Dify, Jina, Mistral, MokaAI, Palm, Tencent, Xunfei, and Zhipu generic unsupported adaptor methods now use typed unsupported-feature errors.
  - Done third batch: DeepSeek, Gemini, Minimax, Moonshot, Ollama, Perplexity, and SiliconFlow unsupported adaptor methods now use typed unsupported-feature errors.
  - Done fourth batch: AWS, Baidu v2, Claude, Coze, Jimeng, Replicate, Vertex, Volcengine, and Zhipu 4V unsupported adaptor methods now use typed unsupported-feature errors; old provider `TODO implement me` / `errors.New("not implemented")` scan is clean.
- [x] Refresh non-MCP backlog audit after adaptor error hygiene.
  - Acceptance: remaining provider `TODO` / `not implemented` scan findings are either migrated, intentionally deferred, or documented with explicit rationale.
  - Done: `docs/non-mcp-backlog-audit.md` now reflects completed provider unsupported migration and identifies Coze content handling, Cohere stream usage, API version normalization, and DTO shape tests as the next non-MCP backlog.
- [x] Audit Coze non-text content handling.
  - Acceptance: Coze response conversion has tests for known text, image/file or unknown content blocks, and behavior is deterministic for unsupported content.
  - Done: Coze request conversion now maps OpenAI string/text content to Coze text, image/file/video content to `object_string` payloads, and unsupported media to deterministic text placeholders; tests cover text, image, file, video, and unsupported mixed-content paths.
- [x] Verify Cohere streaming usage behavior.
  - Acceptance: Cohere stream usage fallback is tested and documented, or fixed if stream chunks expose usage metadata.
  - Done: Cohere stream final responses now use upstream `response.meta.billed_units` when present, always compute `total_tokens`, fill missing prompt/completion fields from local estimates only when needed, and emit a final usage chunk before `[DONE]` when usage output is enabled; tests cover upstream usage, fallback usage, partial metadata, and emitted stream usage.
- [x] Review API version normalization.
  - Acceptance: `middleware/distributor.go` API version behavior is traced with tests before any normalization logic changes.
  - Done: provider metadata setup is now isolated in `setupProviderMetadataContext`; tests lock the current Azure/Xunfei/Gemini/Cloudflare/MokaAI `api_version`, Vertex `region`, Ali `plugin`, Coze `bot_id`, and OpenAI no-op behavior, plus `GetAPIVersion` query-over-context precedence. No provider behavior was changed.
- [x] Add DTO shape compatibility tests.
  - Acceptance: audio stream lifecycle and Gemini thinking-budget conflict TODOs are covered by focused DTO/provider compatibility tests.
  - Done: audio request tests lock `stream_format=sse` as the only stream trigger and document that boolean `stream` is ignored; Gemini generation config tests lock budget+level coexistence, snake_case parsing, and snake-over-camel precedence. The DTO TODO comments were replaced with explicit compatibility notes.

## P0 - Bridge daemon verification

- [x] Expand the real Bridge daemon concurrency smoke to cover `remote_edit` and `remote_glob`.
  - Acceptance: `make mcp-bridge-smoke` exercises write/read/edit/glob/grep/tree/MCP proxy calls through `/mcp/v1`.
  - Acceptance: every added call is persisted in `mcp_tool_calls`, `bridge_audit_logs`, and the local daemon JSONL audit.
- [x] Add daemon negative-path smoke coverage for write-disabled clients.
  - Acceptance: a daemon started without `--enable-write` rejects `remote_write` with `REMOTE_WRITE_DISABLED`.
  - Acceptance: server records error status and refund path for the failed MCP call.
- [x] Add daemon MCP target policy smoke coverage.
  - Acceptance: non-loopback MCP targets are rejected by default with `MCP_PROXY_FORBIDDEN_TARGET`.
  - Acceptance: policy relaxation through `--allow-non-loopback-mcp` is documented but not enabled by default in smoke.

## P1 - Bridge daemon hardening

- [x] Add a lightweight daemon self-test mode that exercises file guards without connecting to data-proxy.
  - Acceptance: `node tools/bridge_client_daemon.mjs --self-test --workspace=<tmp>` exits 0 and covers path traversal/write-disabled checks.
- [x] Add structured reconnect counters to daemon local audit events.
  - Acceptance: audit JSONL includes reconnect attempt, delay, open/clean-close status, and server-close reason.
- [x] Add configurable scan/result limits to daemon CLI.
  - Acceptance: limits can be set by flags and are reflected in `remote_env_info` metadata.

## P1 - MCP Proxy / OpenAPI robustness

- [x] Add MCP Proxy bridge tests for session replacement while a call is pending.
  - Acceptance: pending calls fail with `BRIDGE_CLIENT_DISCONNECTED` or timeout consistently and refund billing.
- [x] Add OpenAPI binary object authorization regression tests for expired/foreign download links.
  - Acceptance: owner/admin can download valid links; other users and expired links are rejected.
- [x] Add schema de-duplication metrics to OpenAPI import preview.
  - Acceptance: preview shows reused schema count and imported tool count.

## P2 - Operations Dashboard polish

- [x] Add a compact dashboard smoke route test for MCP navigation sections.
  - Acceptance: generated route tree includes MCP index and section routes.
- [x] Add trend panel empty-state and partial-data handling tests.
  - Acceptance: no runtime error when trend endpoints return empty arrays.

## P0 - Regression and release guardrails

- [x] Add a unified MCP regression Make target.
  - Acceptance: `make mcp-regression` runs OpenAPI, MCP Proxy, Bridge daemon, and MCP Dashboard checks.
  - Acceptance: failing sections are split into reusable targets for quick diagnosis.
- [x] Add cross-database MCP migration regression documentation and opt-in targets.
  - Acceptance: SQLite default, MySQL opt-in, and PostgreSQL opt-in commands cover MCP, Bridge, billing event, and OpenAPI binary object tables.

## P1 - Core capability expansion

- [x] Add server-side Bridge client policy controls.
  - Acceptance: admins can configure allowed tools, write permission, max result size, scan limits, and MCP target allowlist per Bridge client.
  - Acceptance: daemon defaults remain conservative and server-side policy is enforced before tool forwarding.
- [x] Implement MCP Proxy OAuth authentication support.
  - Acceptance: MCP Proxy supports OAuth token resolve/cache/refresh without regressing none/bearer/basic/header auth.
  - Acceptance: auth failures are observable in discovery events and health checks without leaking secrets.
- [x] Surface OpenAPI import schema metrics and diff summary in the MCP Dashboard.
  - Acceptance: import preview displays importable tool count, schema count, reused schema count, skipped reasons, and diff summary.
- [x] Add OpenAPI binary object management to the MCP Dashboard.
  - Acceptance: admins can view binary object counts, bytes, expiry state, cleanup dry-run, cleanup execute, and download audit context.

## P1 - Reliability and observability

- [x] Add MCP tool call idempotency and replay protection.
  - Acceptance: repeated client request IDs do not double-charge or double-settle a tool call.
- [x] Add Bridge multi-client selection and failover.
  - Acceptance: Bridge MCP Proxy can choose by latest activity/capability and fail over to another eligible online client.
- [x] Add MCP operations review queue.
  - Acceptance: health check, heartbeat, stale Bridge clients, and high-error tools produce actionable review reasons in dashboard summaries.
  - Done: `service/mcp_review.go` aggregates proxy-server review state, stale bridge clients (online in DB but no live hub session), failed health-check/heartbeat runs, and high-error-rate tools into `MCPSummary.review_queue` (admin-wide scope only).
  - Done: MCP Overview shows a Review Queue panel with critical/warning counts and per-item drilldown to proxy servers, bridge clients, and tool calls.
  - Done: severity (`critical`/`warning`), reason codes (`bridge_stale`, `health_check_failed`, `heartbeat_failed`, `high_error_rate_tool`, plus existing proxy reasons) and tests in `service/mcp_review_test.go`.

## P2 - Operations polish

- [x] Expand MCP Overview with Bridge and OpenAPI storage trends.
  - Acceptance: overview shows Bridge online trend, Proxy error topN, OpenAPI binary storage trend, and refund/settlement anomaly summary.
  - Backend: add summary DTO/model/service helpers for Bridge online buckets, Proxy error TopN, OpenAPI binary object storage buckets, and MCP billing anomaly counters.
  - Frontend: add compact Overview panels that tolerate empty/partial trend payloads and drill down to existing sections where possible.
  - Validation: cover backend aggregation with service tests and dashboard normalization/smoke tests.
  - Done: `/api/mcp/summary` includes `operations_trends` with Bridge online/session buckets, OpenAPI binary object storage buckets, Proxy error TopN, and MCP billing anomaly counters.
  - Done: MCP Overview shows storage/Bridge mini trends plus Proxy error and billing anomaly panels with drilldown links.
  - Done: `service/mcp_overview_trends_test.go` and `scripts/check-mcp-trends.mjs` cover empty/partial payloads and aggregate signals.
- [x] Improve billing relation repair UX.
  - Acceptance: admins can preview relation diffs and repair selected items without running a broad backfill.
  - Backend: expose selected-item repair payloads for missing/orphan MCP billing relations and keep broad backfill as a fallback.
  - Frontend: add preview rows with per-item selection, summary counters, and repair action feedback.
  - Validation: cover dry-run, selected repair, idempotent repair, and no-op repair paths.
  - Done: added `POST /api/billing/events/relation-repair` for selected audit relation repair with dry-run, stale payload validation, idempotent existing-link handling, and no-op empty selection behavior.
  - Done: Billing Events Audit Relations panel now stores preview rows, supports per-row/select-all repair selection, and shows created/skipped/invalid repair feedback while keeping broad backfill as fallback.
  - Validation: targeted service/controller/router tests, `npm run typecheck --silent`, `make mcp-dashboard-check`, and `make mcp-regression`.
- [x] Publish MCP/Bridge/OpenAPI runbook.
  - Acceptance: docs cover local daemon, production policies, common error codes, smoke commands, and rollback/cleanup guidance.
  - Docs: include local daemon setup, policy defaults, Bridge failover, OpenAPI binary storage, review queue, billing repair, smoke/regression commands, and rollback cleanup.
  - Validation: link every documented command to an existing Make target or script.
  - Done: added `docs/mcp-bridge-openapi-runbook.md` with operations map, command index, local daemon flow, production policy defaults, OpenAPI binary object handling, review queue triage, billing repair guidance, common error codes, rollback, and cleanup guidance.
  - Done: linked `docs/mcp-bridge-smoke.md` to the new operations runbook.
  - Validation: verified every runbook command maps to an existing Make target or Node script.

## P1 - Admin user quota regression

- [x] Fix quota adjustment mode state leakage across users.
  - Acceptance: selecting override for one user cannot make the next user's adjustment submit override while the dialog appears to be add.
  - Done: classic quota dialog now resets mode and amount on open, cancel, success, and user switch.
  - Done: default quota dialog now resets on close/success, and remounts on open/user changes to start from add mode.
  - Validation: default typecheck passed; targeted default ESLint/prettier passed; classic targeted ESLint passed; `git diff --check` passed.

## P1 - Setup runtime config auto-apply

- [x] Replace the first-run manual restart dead end with container-safe automatic restart.
  - Acceptance: after the setup wizard saves database/Redis runtime config in Docker, Data Proxy schedules a controlled process exit so Compose can restart the container and the wizard can continue when `/api/setup` is healthy again.
  - Acceptance: non-container deployments still keep the explicit manual restart prompt instead of attempting risky hot-reload of global DB/Redis state.
  - Done: `POST /api/setup/runtime-config` now reports restart support/scheduling metadata, schedules one delayed restart in container deployments, and supports `DATA_PROXY_SETUP_AUTO_RESTART=false` as an operations escape hatch.
  - Done: the setup wizard waits for the restarted service to report `runtime_config_restart_required=false`, shows an automatic restart progress state, and refreshes setup status before allowing the next step.
  - Validation: `go test ./controller -run 'TestPostSetupRuntimeConfig'`, `cd web/default && bun run typecheck`, targeted setup ESLint/prettier checks, and `git diff --check` passed.

## P1 - Remove legacy frontend source

- [x] Delete the old UI and keep only the newer frontend.
  - Acceptance: the repository no longer contains the legacy frontend source directory; build, release, deployment, rules, UI plan, and license documentation all point only to `web/default`.
  - Done: removed the old frontend directory, removed classic release workflow build steps, refreshed project rules/docs/license scope, and kept `web/package.json` as a single-workspace frontend root.
  - Validation: `rg` found no remaining legacy UI directory references outside intentional compatibility comments, `cd web && bun install --frozen-lockfile`, `cd web/default && bun run typecheck`, `make build-all-frontends`, and `git diff --check` passed.

## P1 - New UI usability polish

- [x] Clarify external links in the top navigation.
  - Acceptance: the topbar documentation entry shows an external-link affordance on desktop and mobile.
  - Done: added the external-link icon to external top navigation anchors.
- [x] Surface the OpenAI-compatible API Base URL on the API keys page.
  - Acceptance: users can see and copy the active `/v1` Base URL without leaving the API keys workflow.
  - Done: added a compact Base URL strip with one-click copy, using configured server address with a browser-origin fallback.
- [x] Expand command search beyond menu entries.
  - Acceptance: search can find common settings fields such as system name, logo URL, footer, home page content, notice, API info, and announcements.
  - Done: added settings-field search entries that route to the matching settings sections.
- [x] Support logo upload from the system information settings.
  - Acceptance: root users can upload PNG, JPG, WebP, GIF, or ICO logos up to 5MB and receive a public URL that can be saved as the logo URL.
  - Done: added a root-only `/api/uploads/system/logo` endpoint, static serving for uploaded system assets, and an upload action beside the logo URL field.
- [x] Polish admin/MCP translations and dense tab layouts.
  - Acceptance: untranslated MCP/Admin UI strings are localized and crowded horizontal tabs remain usable on narrower screens.
  - Done: expanded static i18n keys/locales and changed crowded settings tabs to horizontal scrolling layouts.
  - Validation: `go test ./controller ./router`, `cd web/default && bun run typecheck`, and `git diff --check` passed.

## Done

- [x] Commit MCP/Bridge, OpenAPI, billing events, wallet ledger, and operations dashboard checkpoint.
- [x] Add real local Bridge daemon for data-proxy integration testing.
- [x] Add concurrent daemon smoke and stress targets.
- [x] Verify daemon reconnect after server-initiated session close.
