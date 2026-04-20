#!/usr/bin/env bash

shared_ops_token_normalize() {
  local token="${1:-}"

  token="${token//$'\r'/}"
  token="${token//$'\n'/}"
  token="${token#"${token%%[![:space:]]*}"}"
  token="${token%"${token##*[![:space:]]}"}"

  case "${token}" in
    ""|placeholder|PLACEHOLDER|REPLACE_WITH_*)
      return 1
      ;;
  esac

  printf '%s\n' "${token}"
}

shared_ops_token_remote_shared_dir() {
  if [[ -n "${TARS_REMOTE_SHARED_DIR:-}" ]]; then
    printf '%s\n' "${TARS_REMOTE_SHARED_DIR}"
    return 0
  fi

  printf '%s/team-shared\n' "${TARS_REMOTE_BASE_DIR:-/data/tars-setup-lab}"
}

shared_ops_token_remote_user() {
  local remote_user="${TARS_REMOTE_USER:-}"
  local remote_host="${TARS_REMOTE_HOST:-192.168.3.100}"

  if [[ -n "${remote_user}" ]]; then
    printf '%s\n' "${remote_user}"
    return 0
  fi

  if [[ "${remote_host}" == "192.168.3.100" ]]; then
    printf 'root\n'
    return 0
  fi

  return 1
}

shared_ops_token_fetch_remote() {
  local remote_user
  local remote_host="${TARS_REMOTE_HOST:-192.168.3.100}"
  local remote_shared_dir

  remote_user="$(shared_ops_token_remote_user 2>/dev/null || true)"
  if [[ -z "${remote_user}" ]]; then
    return 1
  fi

  remote_shared_dir="$(shared_ops_token_remote_shared_dir)"
  ssh -o BatchMode=yes -o ConnectTimeout=5 "${remote_user}@${remote_host}" \
    "sed -n 's/^TARS_OPS_API_TOKEN=//p' '${remote_shared_dir}/shared-test.env' | head -n 1"
}

shared_ops_token_resolve() {
  local token="${TARS_OPS_API_TOKEN:-}"

  token="$(shared_ops_token_normalize "${token}" 2>/dev/null || true)"
  if [[ -n "${token}" ]]; then
    printf '%s\n' "${token}"
    return 0
  fi

  token="$(shared_ops_token_fetch_remote 2>/dev/null || true)"
  token="$(shared_ops_token_normalize "${token}" 2>/dev/null || true)"
  if [[ -z "${token}" ]]; then
    return 1
  fi

  printf '%s\n' "${token}"
}

shared_ops_token_local_override() {
  shared_ops_token_normalize "${TARS_OPS_API_TOKEN:-}"
}

shared_ops_token_export() {
  local token

  token="$(shared_ops_token_resolve)" || return 1
  export TARS_OPS_API_TOKEN="${token}"
  printf '%s\n' "${token}"
}
