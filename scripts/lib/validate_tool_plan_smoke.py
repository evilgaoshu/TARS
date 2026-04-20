#!/usr/bin/env python3

import argparse
import json
import re
from pathlib import Path


SCENARIO_DEFS = {
    "metrics": {
        "required_tools": ["metrics.query_range"],
        "required_attachments": ["metrics-range.json", "metrics-range.png"],
        "artifact_tools": [],
        "summary_tools": ["metrics.query_range"],
    },
    "logs": {
        "required_tools": ["metrics.query_range", "logs.query", "observability.query"],
        "required_attachments": [],
        "artifact_tools": ["logs.query", "observability.query"],
        "summary_tools": ["logs.query", "observability.query"],
    },
    "observability": {
        "required_tools": ["metrics.query_range", "logs.query", "observability.query"],
        "required_attachments": [],
        "artifact_tools": ["logs.query", "observability.query"],
        "summary_tools": ["observability.query", "logs.query"],
    },
    "delivery": {
        "required_tools": ["metrics.query_range", "logs.query", "observability.query", "delivery.query"],
        "required_attachments": [],
        "artifact_tools": ["logs.query", "observability.query", "delivery.query"],
        "summary_tools": ["delivery.query", "observability.query", "logs.query"],
    },
}


def _as_int(value):
    if isinstance(value, bool):
        return int(value)
    if isinstance(value, (int, float)):
        return int(value)
    try:
        return int(str(value).strip())
    except (TypeError, ValueError):
        return 0


def _compact_output_summary(output):
    if not isinstance(output, dict):
        return ""
    for key in ("summary", "release", "branch"):
        value = str(output.get(key) or "").strip()
        if value:
            return value
    result = output.get("result")
    if isinstance(result, dict):
        for key in ("summary", "release", "branch"):
            value = str(result.get(key) or "").strip()
            if value:
                return value
        if _as_int(result.get("result_count")) > 0:
            return f"result_count={_as_int(result.get('result_count'))}"
    points = _as_int(output.get("points"))
    series = _as_int(output.get("series_count"))
    if points > 0 and series > 0:
        return f"series_count={series}, points={points}"
    if points > 0:
        return f"points={points}"
    if series > 0:
        return f"series_count={series}"
    return ""


def _evidence_tokens(text):
    return [
        token
        for token in re.findall(r"[a-z0-9_./:-]+|[\u4e00-\u9fff]+", text.lower())
        if len(token) >= 4
    ]


def validate_scenario(name, detail):
    if name not in SCENARIO_DEFS:
        raise SystemExit(f"unsupported smoke scenario: {name}")

    scenario = SCENARIO_DEFS[name]
    if detail.get("status") != "resolved":
        raise SystemExit(
            f"scenario={name} expected resolved session, got {detail.get('status')}: {json.dumps(detail, ensure_ascii=False)}"
        )
    if len(detail.get("executions") or []) != 0:
        raise SystemExit(f"scenario={name} expected no executions: {json.dumps(detail, ensure_ascii=False)}")

    summary = str(detail.get("diagnosis_summary") or "").strip()
    if not summary:
        raise SystemExit(f"scenario={name} expected diagnosis summary: {json.dumps(detail, ensure_ascii=False)}")
    if summary.startswith("已分析请求："):
        raise SystemExit(f"scenario={name} expected evidence-aware summary instead of generic fallback: {summary}")
    if "artifact_count=" in summary:
        raise SystemExit(f"scenario={name} expected evidence detail instead of attachment counts: {summary}")

    tools = [(step or {}).get("tool") or "" for step in (detail.get("tool_plan") or [])]
    if "execution.run_command" in tools:
        raise SystemExit(f"scenario={name} should not include execution.run_command: {tools}")
    expected_tools = scenario["required_tools"]
    if tools[: len(expected_tools)] != expected_tools:
        raise SystemExit(f"scenario={name} expected tools {expected_tools}, got {tools}")

    steps_by_tool = {}
    for step in detail.get("tool_plan") or []:
        tool = (step or {}).get("tool") or ""
        steps_by_tool.setdefault(tool, step or {})

    for tool in expected_tools:
        step = steps_by_tool.get(tool) or {}
        if step.get("status") != "completed":
            raise SystemExit(
                f"scenario={name} expected completed step for {tool}, got {step.get('status')}: {json.dumps(step, ensure_ascii=False)}"
            )

    attachment_names = [(item or {}).get("name") or "" for item in (detail.get("attachments") or [])]
    for expected_name in scenario["required_attachments"]:
        if expected_name not in attachment_names:
            raise SystemExit(f"scenario={name} expected attachment {expected_name}, got {attachment_names}")

    for tool in scenario["artifact_tools"]:
        step = steps_by_tool.get(tool) or {}
        output = step.get("output") or {}
        if _as_int(output.get("artifact_count")) <= 0:
            raise SystemExit(
                f"scenario={name} expected artifact_count>0 for {tool}, got {json.dumps(step, ensure_ascii=False)}"
            )

    summary_lower = summary.lower()
    evidence_candidates = []
    for tool in scenario["summary_tools"]:
        step = steps_by_tool.get(tool) or {}
        candidate = _compact_output_summary(step.get("output") or {})
        if candidate:
            evidence_candidates.append(candidate)
        if tool.lower() in summary_lower:
            return

    for candidate in evidence_candidates:
        if candidate in summary:
            return
        for token in _evidence_tokens(candidate):
            if token in summary_lower:
                return

    raise SystemExit(
        f"scenario={name} expected summary to reflect completed evidence, got {summary}; candidates={evidence_candidates}"
    )


def main():
    parser = argparse.ArgumentParser(description="Validate tool-plan smoke session detail")
    parser.add_argument("--scenario", required=True)
    parser.add_argument("--detail-file", required=True)
    args = parser.parse_args()

    detail = json.loads(Path(args.detail_file).read_text(encoding="utf-8"))
    validate_scenario(args.scenario, detail)


if __name__ == "__main__":
    main()
