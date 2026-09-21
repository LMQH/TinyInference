SHELL := /bin/sh
COMPOSE := docker compose --project-name mini-inference --env-file config/runtime.mac.conf

.PHONY: runtime-build runtime-start runtime-stop build-images resolve-config render migrate start status logs stop lifecycle-proof privacy-proof backup backup-prune restore-drill restore-record restore-revoke-proof restore-clean

runtime-build:
	./ops/runtime/build.sh

runtime-start:
	./ops/runtime/start.sh

runtime-stop:
	./ops/runtime/stop.sh

build-images:
	./ops/images/build.sh

resolve-config:
	./ops/config/resolve.sh

render:
	$(COMPOSE) config

migrate:
	$(COMPOSE) up -d postgres
	$(COMPOSE) --profile ops run --rm migrate

start: runtime-start
	$(COMPOSE) up -d postgres backup-scheduler controller api web

status:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs --no-color --since 30m api web controller backup-scheduler postgres

stop:
	$(COMPOSE) stop web api controller backup-scheduler postgres
	./ops/runtime/stop.sh

lifecycle-proof:
	./ops/lifecycle-proof/host-gate.sh

privacy-proof:
	./ops/privacy-proof/run.sh

backup:
	$(COMPOSE) exec backup-scheduler /opt/mini-inference/jobs/backup/backup.sh cycle

backup-prune:
	$(COMPOSE) --profile ops run --rm backup-prune

restore-drill:
	python3 ops/restore-drill/run.py

restore-record:
	@set +e; revoke_done=0; \
	cleanup() { \
		if [ "$$revoke_done" -eq 0 ]; then \
			$(COMPOSE) --profile restore run --rm restore-proof-revoke; cleanup_status=$$?; \
			[ "$$cleanup_status" -eq 0 ] || exit "$$cleanup_status"; \
		fi; \
	}; \
	trap cleanup EXIT; \
	trap 'exit 129' HUP; trap 'exit 130' INT; trap 'exit 143' TERM; \
	$(COMPOSE) --profile restore run --rm restore-proof-grant; grant_status=$$?; \
	record_status=$$grant_status; \
	if [ "$$grant_status" -eq 0 ]; then \
		$(COMPOSE) --profile restore run --rm restore-proof-recorder; record_status=$$?; \
	fi; \
	$(COMPOSE) --profile restore run --rm restore-proof-revoke; revoke_status=$$?; \
	revoke_done=1; trap - EXIT HUP INT TERM; \
	[ "$$revoke_status" -eq 0 ] || exit "$$revoke_status"; \
	exit "$$record_status"


restore-revoke-proof:
	$(COMPOSE) --profile restore run --rm restore-proof-revoke

restore-clean:
	$(COMPOSE) --profile restore rm -sfv postgres-restore restore-evidence-init restore-drill restore-proof-grant restore-proof-recorder restore-proof-revoke
	docker volume rm mini-inference_restore-data mini-inference_restore-evidence
