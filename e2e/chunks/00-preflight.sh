#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"

chunk_begin "00" "Preflight and services"

check_tooling() {
	assert_command_exists "go is on PATH" go
	assert_command_exists "curl is on PATH" curl
	assert_command_exists "git is on PATH" git
	assert_command_exists "docker is on PATH" docker

	local version
	version="$(go_version)"
	if version_at_least "$version" "1.25.0"; then
		check_passed "go is 1.25 or newer"
	else
		check_failed "go is 1.25 or newer" "found $version"
	fi
}

check_docker_is_running() {
	if services_available; then
		check_passed "the docker daemon is reachable"
		return 0
	fi
	check_failed "the docker daemon is reachable" \
		"docker info failed; start Docker Desktop and run this again"
	chunk_end
}

check_the_port_is_free() {
	if [ -z "$(lsof -ti:"$E2E_PORT" 2>/dev/null)" ]; then
		check_passed "port $E2E_PORT is free"
		return 0
	fi
	lsof -ti:"$E2E_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
	sleep 1
	if [ -z "$(lsof -ti:"$E2E_PORT" 2>/dev/null)" ]; then
		check_skipped "port $E2E_PORT is free" "a stale listener was killed first"
		return 0
	fi
	check_failed "port $E2E_PORT is free" "still held by $(lsof -ti:"$E2E_PORT" | tr '\n' ' ')"
}

check_services_come_up() {
	if services_up; then
		check_passed "docker compose brings postgres and redis up"
	else
		check_failed "docker compose brings postgres and redis up" \
			"$(compose ps 2>&1 | head -10)"
		chunk_end
	fi

	if postgres_ready; then
		check_passed "postgres accepts connections on $SHOP_DB_PORT"
	else
		check_failed "postgres accepts connections on $SHOP_DB_PORT" "$(compose logs postgres 2>&1 | tail -10)"
	fi

	if redis_ready; then
		check_passed "redis answers PING on $SHOP_REDIS_ADDR"
	else
		check_failed "redis answers PING on $SHOP_REDIS_ADDR" "$(compose logs redis 2>&1 | tail -10)"
	fi
}

record_environment() {
	note "commit" "$(repository_commit)"
	note "platform" "$(platform_name)"
	note "go" "$(go_version)"
	note "docker" "$(docker --version 2>/dev/null | sed 's/Docker version //')"
	note "compose" "$(docker compose version --short 2>/dev/null)"
	note "postgres" "$(postgres_query 'SHOW server_version;')"
	note "redis" "$(redis_command INFO server 2>/dev/null | sed -n 's/^redis_version:\(.*\)$/\1/p')"
	if repository_is_dirty; then
		note "working tree" "dirty, so this run does not describe a clean commit"
	else
		note "working tree" "clean"
	fi
}

check_tooling
check_docker_is_running
check_the_port_is_free
check_services_come_up
record_environment

chunk_end
