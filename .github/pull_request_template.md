## Change Summary

- Touched layer(s):
- Boundary impact:
- User/API impact:

## Architecture Checklist

- [ ] Template identity has been replaced for derived service repos, or this PR intentionally targets the template repo.
- [ ] Domain code does not import Gin, GORM, delivery DTOs, adapter models, `conf`, `di`, `server`, `workers`, or `infrastructures`.
- [ ] Handlers only map request/response and delegate to usecases.
- [ ] Usecases depend on domain contracts, repository ports, and service ports only.
- [ ] Repository adapters map through domain entities/records, not delivery DTOs.
- [ ] Schema/model/entity changes include explicit mapper updates and `mappingtest` coverage.
- [ ] Multi-write flows use `repositories.TransactionManager`.
- [ ] No handwritten fake/stub/mock/spy test doubles were added.

## Validation

- [ ] `make mockgen`
- [ ] `make template-identity-check`
- [ ] `go test ./...`
- [ ] `make lint`
- [ ] `git diff --check`

## Operational Notes

- [ ] Config/env changes documented or not needed.
- [ ] DB migration added or not needed.
- [ ] Backward compatibility checked or not relevant.
- [ ] Logs/errors are structured and do not leak sensitive details.
