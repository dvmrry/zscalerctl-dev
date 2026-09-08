#!/usr/bin/env python3
"""Evaluate submitted agent traces against the synthetic workflow tasks.

The evaluator is intentionally offline.  It only reads JSON supplied by the
caller and compares the trace with the versioned task expectations.  It never
executes zscalerctl, contacts a tenant, or invokes a model API.
"""

from __future__ import annotations

import argparse
import json
import math
import re
import sys
from pathlib import Path
from typing import Any, Iterable


EVALUATOR_VERSION = "zscalerctl.agent-workflows.evaluator/v1"
BASELINE = "not_yet_measured"
TRACE_CONTRACT = "zscalerctl.agent-workflows.trace"
TASK_CONTRACT = "zscalerctl.agent-workflows.tasks"
CORPUS_CONTRACT = "zscalerctl.agent-workflows.corpus"
FIELD_STATES = {"renderable", "known_but_not_renderable", "secret"}
RESOURCE_KINDS = {"collection", "singleton"}
RULES = {
    "known_id_direct_get",
    "unknown_field_refusal",
    "unavailable_field_is_not_false",
    "bounded_large_list",
    "saved_collection_repeated_reads",
    "collection_failure_is_not_empty",
}


class InputError(ValueError):
    """Raised when a submitted contract document is malformed."""


def load_json(path: str | Path) -> Any:
    """Load one UTF-8 JSON document without interpreting any external data."""

    try:
        with Path(path).open("r", encoding="utf-8") as handle:
            return json.load(handle)
    except (OSError, json.JSONDecodeError) as exc:
        raise InputError(f"cannot read JSON {path}: {exc}") from exc


def _require_dict(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise InputError(f"{label} must be an object")
    return value


def _require_string(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value:
        raise InputError(f"{label} must be a non-empty string")
    return value


def _require_bool(value: Any, label: str) -> bool:
    if not isinstance(value, bool):
        raise InputError(f"{label} must be boolean")
    return value


def _require_int(value: Any, label: str, minimum: int = 0) -> int:
    if not isinstance(value, int) or isinstance(value, bool) or value < minimum:
        raise InputError(f"{label} must be an integer >= {minimum}")
    return value


def _require_number(value: Any, label: str, minimum: int | float = 0) -> int | float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise InputError(f"{label} must be a finite number >= {minimum}")
    if isinstance(value, float) and not math.isfinite(value):
        raise InputError(f"{label} must be a finite number >= {minimum}")
    if value < minimum:
        raise InputError(f"{label} must be a finite number >= {minimum}")
    return value


def _require_list(value: Any, label: str, *, non_empty: bool = False) -> list[Any]:
    if not isinstance(value, list) or (non_empty and not value):
        suffix = " non-empty" if non_empty else ""
        raise InputError(f"{label} must be a{suffix} array")
    return value


def _require_enum(value: Any, label: str, allowed: set[str]) -> str:
    value = _require_string(value, label)
    if value not in allowed:
        choices = ", ".join(sorted(allowed))
        raise InputError(f"{label} must be one of: {choices}")
    return value


def _reject_extra_keys(value: dict[str, Any], allowed: set[str], label: str) -> None:
    extras = sorted(set(value) - allowed)
    if extras:
        raise InputError(f"{label} has unsupported fields: {', '.join(extras)}")


def _validate_constraints(task_id: str, constraints: dict[str, Any]) -> None:
    """Validate every constraint consumed by scoring before rule dispatch."""

    for key in (
        "max_live_reads",
        "max_dump_reads",
        "max_response_records",
        "max_answer_records",
        "max_record_fields",
    ):
        if key in constraints:
            _require_int(constraints[key], f"task {task_id}.constraints.{key}")
    for key in (
        "required_live_get",
        "forbid_live_list_before_known_get",
        "required_refusal",
        "forbid_false_for_unavailable",
        "required_bounded_page",
        "required_failed_list",
        "forbid_success_empty",
    ):
        if key in constraints:
            _require_bool(constraints[key], f"task {task_id}.constraints.{key}")
    if "required_limit" in constraints:
        _require_int(constraints["required_limit"], f"task {task_id}.constraints.required_limit", 1)
    if "required_dump_id" in constraints:
        _require_string(constraints["required_dump_id"], f"task {task_id}.constraints.required_dump_id")
    if "required_read_sequence" in constraints:
        sequence = _require_list(
            constraints["required_read_sequence"],
            f"task {task_id}.constraints.required_read_sequence",
            non_empty=True,
        )
        for index, operation in enumerate(sequence):
            _require_enum(
                operation,
                f"task {task_id}.constraints.required_read_sequence[{index}]",
                {"list", "get", "show"},
            )


def _validate_common_fixture(task_id: str, fixture: dict[str, Any]) -> None:
    product = _require_string(fixture.get("product"), f"task {task_id}.fixture.product")
    resource_name = _require_string(fixture.get("resource_name"), f"task {task_id}.fixture.resource_name")
    resource = _require_string(fixture.get("resource"), f"task {task_id}.fixture.resource")
    if resource != f"{product}.{resource_name}":
        raise InputError(f"task {task_id}.fixture.resource must be {product}.{resource_name}")


def _validate_task_rule(task: dict[str, Any]) -> None:
    """Check rule-specific manifest data so evaluation cannot hit KeyError."""

    task_id = task["id"]
    fixture = task["fixture"]
    expected = task["expected"]
    constraints = task["constraints"]
    if task["rule"] != "saved_collection_repeated_reads":
        _validate_common_fixture(task_id, fixture)
    _validate_constraints(task_id, constraints)
    _require_string(task.get("prompt"), f"task {task_id}.prompt")

    if task["rule"] == "known_id_direct_get":
        _require_string(fixture.get("known_id"), f"task {task_id}.fixture.known_id")
        fields = _require_list(fixture.get("fields"), f"task {task_id}.fixture.fields", non_empty=True)
        for index, field in enumerate(fields):
            _require_string(field, f"task {task_id}.fixture.fields[{index}]")
        _require_enum(expected.get("answer_status"), f"task {task_id}.expected.answer_status", {"ok"})
        if not isinstance(expected.get("record"), dict) or not expected["record"]:
            raise InputError(f"task {task_id}.expected.record must be a non-empty object")
        if expected["record"].get("id") != fixture["known_id"]:
            raise InputError(f"task {task_id}.expected.record.id must equal fixture.known_id")
        if set(expected["record"]) != set(fields):
            raise InputError(f"task {task_id}.expected.record fields must match fixture.fields")
        return

    if task["rule"] == "unknown_field_refusal":
        _require_string(fixture.get("requested_field"), f"task {task_id}.fixture.requested_field")
        _require_enum(fixture.get("operation"), f"task {task_id}.fixture.operation", {"list", "get", "show"})
        _require_enum(expected.get("answer_status"), f"task {task_id}.expected.answer_status", {"refused"})
        _require_string(expected.get("error_kind"), f"task {task_id}.expected.error_kind")
        _require_string(expected.get("field"), f"task {task_id}.expected.field")
        if expected["field"] != fixture["requested_field"]:
            raise InputError(f"task {task_id}.expected.field must equal fixture.requested_field")
        return

    if task["rule"] == "unavailable_field_is_not_false":
        _require_string(fixture.get("known_id"), f"task {task_id}.fixture.known_id")
        _require_string(fixture.get("requested_field"), f"task {task_id}.fixture.requested_field")
        _require_string(fixture.get("field_state"), f"task {task_id}.fixture.field_state")
        _require_enum(expected.get("answer_status"), f"task {task_id}.expected.answer_status", {"unavailable"})
        _require_string(expected.get("field"), f"task {task_id}.expected.field")
        _require_string(expected.get("availability"), f"task {task_id}.expected.availability")
        if expected["field"] != fixture["requested_field"]:
            raise InputError(f"task {task_id}.expected.field must equal fixture.requested_field")
        if expected["availability"] != fixture["field_state"]:
            raise InputError(f"task {task_id}.expected.availability must equal fixture.field_state")
        if "value" not in expected or expected["value"] is not None:
            raise InputError(f"task {task_id}.expected.value must be null")
        if not isinstance(expected.get("record"), dict) or not expected["record"]:
            raise InputError(f"task {task_id}.expected.record must be a non-empty object")
        return

    if task["rule"] == "bounded_large_list":
        if fixture.get("large_collection") is not True:
            raise InputError(f"task {task_id}.fixture.large_collection must be true")
        limit = _require_int(fixture.get("limit"), f"task {task_id}.fixture.limit", 1)
        _require_int(fixture.get("offset"), f"task {task_id}.fixture.offset")
        _require_enum(expected.get("answer_status"), f"task {task_id}.expected.answer_status", {"ok"})
        page = expected.get("page")
        if not isinstance(page, dict) or not isinstance(page.get("records"), list) or not page["records"]:
            raise InputError(f"task {task_id}.expected.page.records must be a non-empty array")
        for index, record in enumerate(page["records"]):
            _require_dict(record, f"task {task_id}.expected.page.records[{index}]")
        if not isinstance(page.get("pagination"), dict):
            raise InputError(f"task {task_id}.expected.page.pagination must be an object")
        if constraints.get("required_limit") != limit:
            raise InputError(f"task {task_id}.constraints.required_limit must equal fixture.limit")
        pagination = page["pagination"]
        for key in ("matched_count", "returned_count"):
            _require_int(pagination.get(key), f"task {task_id}.expected.page.pagination.{key}")
        _require_bool(pagination.get("has_more"), f"task {task_id}.expected.page.pagination.has_more")
        if "next_offset" not in pagination:
            raise InputError(f"task {task_id}.expected.page.pagination.next_offset is required")
        if pagination["next_offset"] is not None:
            _require_int(pagination["next_offset"], f"task {task_id}.expected.page.pagination.next_offset")
        _require_bool(
            pagination.get("collection_complete"),
            f"task {task_id}.expected.page.pagination.collection_complete",
        )
        _require_int(pagination.get("limit"), f"task {task_id}.expected.page.pagination.limit", 1)
        _require_int(pagination.get("offset"), f"task {task_id}.expected.page.pagination.offset")
        if pagination.get("limit") != limit or pagination.get("offset") != fixture["offset"]:
            raise InputError(f"task {task_id}.expected.page pagination must match fixture bounds")
        if len(page["records"]) != limit:
            raise InputError(f"task {task_id}.expected.page.records length must equal fixture.limit")
        return

    if task["rule"] == "saved_collection_repeated_reads":
        _require_string(fixture.get("dump_id"), f"task {task_id}.fixture.dump_id")
        _require_string(fixture.get("dump_path"), f"task {task_id}.fixture.dump_path")
        reads = _require_list(fixture.get("reads"), f"task {task_id}.fixture.reads", non_empty=True)
        for index, read_value in enumerate(reads):
            read = _require_dict(read_value, f"task {task_id}.fixture.reads[{index}]")
            _require_enum(read.get("operation"), f"task {task_id}.fixture.reads[{index}].operation", {"list", "get", "show"})
            _require_string(read.get("product"), f"task {task_id}.fixture.reads[{index}].product")
            _require_string(read.get("resource_name"), f"task {task_id}.fixture.reads[{index}].resource_name")
            if read["operation"] == "get":
                _require_string(read.get("id"), f"task {task_id}.fixture.reads[{index}].id")
            elif "id" in read:
                raise InputError(f"task {task_id}.fixture.reads[{index}].id is only valid for get")
        _require_enum(expected.get("answer_status"), f"task {task_id}.expected.answer_status", {"ok"})
        observations = _require_list(expected.get("observations"), f"task {task_id}.expected.observations", non_empty=True)
        if len(observations) != len(reads):
            raise InputError(f"task {task_id}.expected.observations must match fixture.reads length")
        for index, observation_value in enumerate(observations):
            observation = _require_dict(observation_value, f"task {task_id}.expected.observations[{index}]")
            _require_enum(observation.get("operation"), f"task {task_id}.expected.observations[{index}].operation", {"list", "get", "show"})
            _require_string(observation.get("resource"), f"task {task_id}.expected.observations[{index}].resource")
            read = reads[index]
            if observation["operation"] != read["operation"] or observation["resource"] != f"{read['product']}.{read['resource_name']}":
                raise InputError(f"task {task_id}.expected.observations[{index}] must match fixture.reads[{index}]")
            if read["operation"] == "list":
                if "record_ids" not in observation:
                    raise InputError(f"task {task_id}.expected.observations[{index}] needs record_ids for list")
                record_ids = _require_list(observation["record_ids"], f"task {task_id}.expected.observations[{index}].record_ids")
                for record_index, record_id in enumerate(record_ids):
                    _require_string(record_id, f"task {task_id}.expected.observations[{index}].record_ids[{record_index}]")
            elif not isinstance(observation.get("record"), dict):
                raise InputError(f"task {task_id}.expected.observations[{index}] needs record for {read['operation']}")
        if constraints.get("required_dump_id") != fixture["dump_id"]:
            raise InputError(f"task {task_id}.constraints.required_dump_id must equal fixture.dump_id")
        if constraints.get("required_read_sequence") != [read["operation"] for read in reads]:
            raise InputError(f"task {task_id}.constraints.required_read_sequence must match fixture.reads")
        return

    if task["rule"] == "collection_failure_is_not_empty":
        _require_string(fixture.get("failure_mode"), f"task {task_id}.fixture.failure_mode")
        _require_enum(fixture.get("operation"), f"task {task_id}.fixture.operation", {"list"})
        _require_enum(expected.get("answer_status"), f"task {task_id}.expected.answer_status", {"failed"})
        _require_string(expected.get("error_kind"), f"task {task_id}.expected.error_kind")
        if "records" not in expected:
            raise InputError(f"task {task_id}.expected.records is required")
        if expected["records"] is not None:
            raise InputError(f"task {task_id}.expected.records must be null for a failed collection")
        return

    raise InputError(f"task {task_id} has an unsupported rule")


def validate_tasks(tasks_doc: Any) -> dict[str, Any]:
    """Validate the small task-manifest contract used by this evaluator."""

    document = _require_dict(tasks_doc, "task manifest")
    _reject_extra_keys(
        document,
        {"contract", "version", "status", "corpus", "tasks"},
        "task manifest",
    )
    if document.get("contract") != TASK_CONTRACT:
        raise InputError("task manifest has an unsupported contract")
    if document.get("version") != 1:
        raise InputError("task manifest has an unsupported version")
    if document.get("status") != BASELINE:
        raise InputError("task manifest status must remain not_yet_measured")
    _require_string(document.get("corpus"), "task manifest.corpus")
    tasks = document.get("tasks")
    if not isinstance(tasks, list) or not tasks:
        raise InputError("task manifest tasks must be a non-empty array")
    seen: set[str] = set()
    for index, task_value in enumerate(tasks):
        task = _require_dict(task_value, f"task {index}")
        _reject_extra_keys(
            task,
            {"id", "version", "prompt", "rule", "fixture", "expected", "constraints"},
            f"task {index}",
        )
        task_id = _require_string(task.get("id"), f"task {index}.id")
        if re.fullmatch(r"[a-z0-9_]+", task_id) is None:
            raise InputError(f"task {index}.id must contain only lowercase letters, digits, and underscores")
        if task_id in seen:
            raise InputError(f"duplicate task id: {task_id}")
        seen.add(task_id)
        if task.get("version") != 1:
            raise InputError(f"task {task_id} has an unsupported version")
        if task.get("rule") not in RULES:
            raise InputError(f"task {task_id} has an unsupported rule")
        for key in ("fixture", "expected", "constraints"):
            _require_dict(task.get(key), f"task {task_id}.{key}")
        _validate_task_rule(task)
    return document


def _validate_corpus_record(record: Any, fields: dict[str, Any], label: str) -> dict[str, Any]:
    record = _require_dict(record, label)
    for field in record:
        _require_string(field, f"{label} field name")
        if field not in fields:
            raise InputError(f"{label} contains field not in the corpus catalog: {field}")
    return record


def _validate_corpus_resource(resource_key: str, resource: Any) -> dict[str, Any]:
    resource = _require_dict(resource, f"corpus.resources.{resource_key}")
    kind = _require_enum(resource.get("kind"), f"corpus.resources.{resource_key}.kind", RESOURCE_KINDS)
    fields = _require_dict(resource.get("fields"), f"corpus.resources.{resource_key}.fields")
    if not fields:
        raise InputError(f"corpus.resources.{resource_key}.fields must not be empty")
    for field_name, field_spec_value in fields.items():
        _require_string(field_name, f"corpus.resources.{resource_key}.field name")
        field_spec = _require_dict(field_spec_value, f"corpus.resources.{resource_key}.fields.{field_name}")
        _require_enum(
            field_spec.get("state"),
            f"corpus.resources.{resource_key}.fields.{field_name}.state",
            FIELD_STATES,
        )
        _require_string(field_spec.get("type"), f"corpus.resources.{resource_key}.fields.{field_name}.type")

    if kind == "collection":
        records = _require_list(resource.get("records"), f"corpus.resources.{resource_key}.records")
        seen_ids: set[str] = set()
        for index, record_value in enumerate(records):
            record = _validate_corpus_record(record_value, fields, f"corpus.resources.{resource_key}.records[{index}]")
            record_id = _require_string(
                record.get("id"),
                f"corpus.resources.{resource_key}.records[{index}].id",
            )
            if record_id in seen_ids:
                raise InputError(f"corpus.resources.{resource_key} has duplicate record id {record_id}")
            seen_ids.add(record_id)
    else:
        _validate_corpus_record(resource.get("record"), fields, f"corpus.resources.{resource_key}.record")

    if "large_collection" in resource:
        large = _require_dict(resource["large_collection"], f"corpus.resources.{resource_key}.large_collection")
        _require_int(
            large.get("count"),
            f"corpus.resources.{resource_key}.large_collection.count",
            1,
        )
        samples = _require_list(
            large.get("first_page_samples"),
            f"corpus.resources.{resource_key}.large_collection.first_page_samples",
            non_empty=True,
        )
        for index, sample in enumerate(samples):
            _validate_corpus_record(
                sample,
                fields,
                f"corpus.resources.{resource_key}.large_collection.first_page_samples[{index}]",
            )
        if "id_pattern" in large:
            _require_string(large["id_pattern"], f"corpus.resources.{resource_key}.large_collection.id_pattern")
        if "record_template" in large:
            template = _require_dict(
                large["record_template"],
                f"corpus.resources.{resource_key}.large_collection.record_template",
            )
            for field in template:
                if field not in fields:
                    raise InputError(
                        f"corpus.resources.{resource_key}.large_collection.record_template has unknown field {field}"
                    )
    if "dump_records" in resource:
        dump_records = _require_dict(resource["dump_records"], f"corpus.resources.{resource_key}.dump_records")
        for dump_id, records_value in dump_records.items():
            _require_string(dump_id, f"corpus.resources.{resource_key}.dump_records key")
            records = _require_list(
                records_value,
                f"corpus.resources.{resource_key}.dump_records.{dump_id}",
            )
            seen_ids: set[str] = set()
            for index, record_value in enumerate(records):
                record = _validate_corpus_record(
                    record_value,
                    fields,
                    f"corpus.resources.{resource_key}.dump_records.{dump_id}[{index}]",
                )
                if kind == "collection":
                    record_id = _require_string(
                        record.get("id"),
                        f"corpus.resources.{resource_key}.dump_records.{dump_id}[{index}].id",
                    )
                    if record_id in seen_ids:
                        raise InputError(
                            f"corpus.resources.{resource_key}.dump_records.{dump_id} has duplicate id {record_id}"
                        )
                    seen_ids.add(record_id)
            if kind == "singleton" and len(records) != 1:
                raise InputError(
                    f"corpus.resources.{resource_key}.dump_records.{dump_id} must contain one singleton record"
                )
    return resource


def _validate_corpus_dumps(document: dict[str, Any], resources: dict[str, Any]) -> None:
    dumps = _require_dict(document.get("dumps"), "corpus.dumps")
    if not dumps:
        raise InputError("corpus.dumps must not be empty")
    for dump_id, dump_value in dumps.items():
        _require_string(dump_id, "corpus dump id")
        dump = _require_dict(dump_value, f"corpus.dumps.{dump_id}")
        _require_string(dump.get("path"), f"corpus.dumps.{dump_id}.path")
        _require_string(dump.get("redaction"), f"corpus.dumps.{dump_id}.redaction")
        _require_enum(dump.get("status"), f"corpus.dumps.{dump_id}.status", {"complete", "partial"})
        dump_resources = _require_list(
            dump.get("resources"),
            f"corpus.dumps.{dump_id}.resources",
            non_empty=True,
        )
        dump_resource_keys: set[str] = set()
        for index, resource_key in enumerate(dump_resources):
            resource_key = _require_string(resource_key, f"corpus.dumps.{dump_id}.resources[{index}]")
            if resource_key not in resources:
                raise InputError(f"corpus.dumps.{dump_id} references unknown resource {resource_key}")
            dump_resource_keys.add(resource_key)
        allowed_fields = _require_dict(dump.get("allowed_fields"), f"corpus.dumps.{dump_id}.allowed_fields")
        if set(allowed_fields) != dump_resource_keys:
            raise InputError(f"corpus.dumps.{dump_id}.allowed_fields must cover exactly its resources")
        for resource_key, fields_value in allowed_fields.items():
            _require_string(resource_key, f"corpus.dumps.{dump_id}.allowed_fields resource key")
            fields = _require_list(
                fields_value,
                f"corpus.dumps.{dump_id}.allowed_fields.{resource_key}",
                non_empty=True,
            )
            catalog_fields = resources[resource_key]["fields"]
            for index, field in enumerate(fields):
                field = _require_string(
                    field,
                    f"corpus.dumps.{dump_id}.allowed_fields.{resource_key}[{index}]",
                )
                if field not in catalog_fields:
                    raise InputError(
                        f"corpus.dumps.{dump_id}.allowed_fields.{resource_key} references unknown field {field}"
                    )
                if catalog_fields[field]["state"] == "secret":
                    raise InputError(
                        f"corpus.dumps.{dump_id}.allowed_fields.{resource_key} includes secret field {field}"
                    )


def validate_corpus(corpus_doc: Any) -> dict[str, Any]:
    """Validate the fictional corpus and its explicit saved-dump allowlists."""

    document = _require_dict(corpus_doc, "corpus")
    if document.get("contract") != CORPUS_CONTRACT:
        raise InputError("corpus has an unsupported contract")
    if document.get("version") != 1:
        raise InputError("corpus has an unsupported version")
    resources_value = document.get("resources")
    resources = _require_dict(resources_value, "corpus.resources")
    if not resources:
        raise InputError("corpus.resources must not be empty")
    for resource_key, resource in resources.items():
        _require_string(resource_key, "corpus resource key")
        _validate_corpus_resource(resource_key, resource)
    _validate_corpus_dumps(document, resources)
    dumps = document["dumps"]
    for resource_key, resource in resources.items():
        dump_records = resource.get("dump_records", {})
        for dump_id in dump_records:
            if dump_id not in dumps:
                raise InputError(f"corpus.resources.{resource_key}.dump_records references unknown dump {dump_id}")
    for dump_id, dump in dumps.items():
        for resource_key in dump["resources"]:
            resource = resources[resource_key]
            if dump_id not in resource.get("dump_records", {}):
                raise InputError(f"corpus resource {resource_key} has no records for dump {dump_id}")
    return document


def _corpus_resource(corpus: dict[str, Any], resource_key: str, label: str) -> dict[str, Any]:
    resources = corpus["resources"]
    resource = resources.get(resource_key)
    if not isinstance(resource, dict):
        raise InputError(f"{label} references unknown corpus resource {resource_key}")
    return resource


def _source_records(
    resource: dict[str, Any],
    *,
    source: str,
    dump_ref: str | None,
    label: str,
) -> list[dict[str, Any]]:
    if source == "dump":
        if not dump_ref:
            raise InputError(f"{label} dump evidence is missing from_dump")
        dump_records = resource.get("dump_records", {})
        dump_id = dump_ref
        if dump_id not in dump_records:
            raise InputError(f"{label} has no corpus records for dump {dump_ref}")
        records = dump_records[dump_id]
        return records
    if resource["kind"] == "collection":
        return resource["records"]
    record = resource["record"]
    return [record]


def _evidence_records(
    corpus: dict[str, Any],
    resource: dict[str, Any],
    *,
    source: str,
    dump_ref: str | None,
    label: str,
) -> list[dict[str, Any]]:
    if source == "dump":
        if not dump_ref:
            raise InputError(f"{label} dump evidence is missing from_dump")
        dump_id, _ = _dump_config(corpus, dump_ref, label)
        records = resource.get("dump_records", {}).get(dump_id)
        if not isinstance(records, list):
            raise InputError(f"{label} has no corpus records for dump {dump_ref}")
        return records
    if resource["kind"] == "singleton":
        return [resource["record"]]
    records = list(resource["records"])
    large = resource.get("large_collection")
    if isinstance(large, dict):
        records.extend(large["first_page_samples"])
    return records


def _generated_large_record(resource: dict[str, Any], record_id: str) -> dict[str, Any] | None:
    large = resource.get("large_collection")
    if not isinstance(large, dict):
        return None
    pattern = large.get("id_pattern")
    if not isinstance(pattern, str):
        return None
    pattern_re = re.escape(pattern)
    pattern_re = pattern_re.replace(r"\{index:04d\}", r"(?P<index>\d{4})")
    pattern_re = pattern_re.replace(r"\{index\}", r"(?P<index>\d+)")
    match = re.fullmatch(pattern_re, record_id)
    if match is None:
        return None
    index = int(match.group("index"))
    if index < 1 or index > large["count"]:
        return None
    template = large.get("record_template")
    if not isinstance(template, dict):
        return None
    generated: dict[str, Any] = {"id": record_id}
    for field, value in template.items():
        if isinstance(value, str):
            try:
                generated[field] = value.format(index=index)
            except (IndexError, KeyError, ValueError):
                return None
        else:
            generated[field] = value
    return generated


def _dump_config(corpus: dict[str, Any], dump_ref: str, label: str) -> tuple[str, dict[str, Any]]:
    for dump_id, dump_value in corpus["dumps"].items():
        if dump_ref == dump_id or dump_ref == dump_value.get("path"):
            return dump_id, dump_value
    raise InputError(f"{label} references unknown dump {dump_ref}")


def _allowed_fields(
    corpus: dict[str, Any],
    resource_key: str,
    *,
    source: str,
    dump_ref: str | None,
    label: str,
) -> set[str]:
    resource = _corpus_resource(corpus, resource_key, label)
    fields = resource["fields"]
    if source == "dump":
        if not dump_ref:
            raise InputError(f"{label} dump evidence is missing from_dump")
        dump_id, dump = _dump_config(corpus, dump_ref, label)
        allowed = dump["allowed_fields"].get(resource_key)
        if not isinstance(allowed, list):
            raise InputError(f"{label} dump {dump_id} does not contain resource {resource_key}")
        return set(allowed)
    return {field for field, spec in fields.items() if spec["state"] == "renderable"}


def _validate_record_fields(
    corpus: dict[str, Any],
    resource_key: str,
    record: dict[str, Any],
    *,
    source: str,
    dump_ref: str | None,
    label: str,
) -> None:
    resource = _corpus_resource(corpus, resource_key, label)
    fields = resource["fields"]
    allowed = _allowed_fields(corpus, resource_key, source=source, dump_ref=dump_ref, label=label)
    for field in record:
        spec = fields.get(field)
        if not isinstance(spec, dict):
            raise InputError(f"{label} contains unknown corpus field {field}")
        if spec["state"] == "secret":
            raise InputError(f"{label} contains secret field {field}")
        if field not in allowed:
            raise InputError(f"{label} contains field {field} unavailable from {source} source")
    candidates = _evidence_records(
        corpus,
        resource,
        source=source,
        dump_ref=dump_ref,
        label=label,
    )
    if resource["kind"] == "collection":
        record_id = record.get("id")
        if not isinstance(record_id, str) or not record_id:
            raise InputError(f"{label} collection evidence must include a record id")
        candidates = [candidate for candidate in candidates if candidate.get("id") == record_id]
        if not candidates:
            generated = _generated_large_record(resource, record_id)
            if generated is not None:
                candidates = [generated]
    if len(candidates) != 1:
        raise InputError(f"{label} does not identify one corpus record")
    actual = candidates[0]
    for field, value in record.items():
        if field not in actual or actual[field] != value:
            raise InputError(f"{label} field {field} does not match the corpus record")


def _reject_secret_keys(value: Any, corpus: dict[str, Any], label: str) -> None:
    """Reject secret catalog field names wherever submitted JSON places them."""

    if isinstance(value, dict):
        for key, nested in value.items():
            for resource_key, resource in corpus["resources"].items():
                field_spec = resource["fields"].get(key)
                if isinstance(field_spec, dict) and field_spec["state"] == "secret":
                    raise InputError(f"{label} contains secret field {resource_key}.{key}")
            _reject_secret_keys(nested, corpus, label)
    elif isinstance(value, list):
        for nested in value:
            _reject_secret_keys(nested, corpus, label)


def _validate_expected_record(
    corpus: dict[str, Any],
    resource_key: str,
    expected_record: dict[str, Any],
    *,
    source: str,
    dump_ref: str | None,
    record_id: str | None,
    label: str,
) -> None:
    resource = _corpus_resource(corpus, resource_key, label)
    records = _source_records(resource, source=source, dump_ref=dump_ref, label=label)
    if record_id is None:
        if len(records) != 1:
            raise InputError(f"{label} requires a singleton corpus record")
        actual_record = records[0]
    else:
        actual_record = next((record for record in records if record.get("id") == record_id), None)
        if actual_record is None:
            raise InputError(f"{label} references unknown record id {record_id}")
    _validate_record_fields(
        corpus,
        resource_key,
        expected_record,
        source=source,
        dump_ref=dump_ref,
        label=label,
    )
    if not _contains(actual_record, expected_record):
        raise InputError(f"{label} does not match the corpus record")


def validate_task_corpus(tasks: dict[str, Any], corpus: dict[str, Any]) -> None:
    """Ensure task expectations and fixture references are grounded in corpus data."""

    for task in tasks["tasks"]:
        task_id = task["id"]
        fixture = task["fixture"]
        expected = task["expected"]
        rule = task["rule"]
        if rule == "saved_collection_repeated_reads":
            dump_id, dump = _dump_config(corpus, fixture["dump_id"], f"task {task_id}")
            if dump["path"] != fixture["dump_path"]:
                raise InputError(f"task {task_id}.fixture.dump_path does not match corpus dump {dump_id}")
            for index, (read, observation) in enumerate(zip(fixture["reads"], expected["observations"])):
                resource_key = f"{read['product']}.{read['resource_name']}"
                resource = _corpus_resource(corpus, resource_key, f"task {task_id}.fixture.reads[{index}]")
                if read["operation"] == "list":
                    if resource["kind"] != "collection":
                        raise InputError(f"task {task_id}.fixture.reads[{index}] list requires a collection")
                    records = _source_records(
                        resource,
                        source="dump",
                        dump_ref=dump_id,
                        label=f"task {task_id}.fixture.reads[{index}]",
                    )
                    expected_ids = observation.get("record_ids")
                    actual_ids = [record["id"] for record in records]
                    if expected_ids != actual_ids:
                        raise InputError(f"task {task_id}.expected.observations[{index}] does not match the corpus list")
                elif read["operation"] == "get":
                    if resource["kind"] != "collection":
                        raise InputError(f"task {task_id}.fixture.reads[{index}] get requires a collection")
                    _validate_expected_record(
                        corpus,
                        resource_key,
                        observation["record"],
                        source="dump",
                        dump_ref=dump_id,
                        record_id=read["id"],
                        label=f"task {task_id}.expected.observations[{index}]",
                    )
                else:
                    if resource["kind"] != "singleton":
                        raise InputError(f"task {task_id}.fixture.reads[{index}] show requires a singleton")
                    _validate_expected_record(
                        corpus,
                        resource_key,
                        observation["record"],
                        source="dump",
                        dump_ref=dump_id,
                        record_id=None,
                        label=f"task {task_id}.expected.observations[{index}]",
                    )
            continue

        resource_key = fixture["resource"]
        resource = _corpus_resource(corpus, resource_key, f"task {task_id}")
        if rule == "unknown_field_refusal":
            if fixture["operation"] == "list" and resource["kind"] != "collection":
                raise InputError(f"task {task_id} unknown-field list requires a collection resource")
            if fixture["requested_field"] in resource["fields"]:
                raise InputError(f"task {task_id}.fixture.requested_field is present in the corpus catalog")
        elif rule == "known_id_direct_get":
            for field in fixture["fields"]:
                field_spec = resource["fields"].get(field)
                if not isinstance(field_spec, dict) or field_spec["state"] != "renderable":
                    raise InputError(f"task {task_id}.fixture.fields.{field} is not renderable in the corpus")
            _validate_expected_record(
                corpus,
                resource_key,
                expected["record"],
                source="live",
                dump_ref=None,
                record_id=fixture["known_id"],
                label=f"task {task_id}.expected.record",
            )
        elif rule == "unavailable_field_is_not_false":
            field_spec = resource["fields"].get(fixture["requested_field"])
            if not isinstance(field_spec, dict):
                raise InputError(f"task {task_id}.fixture.requested_field is absent from the corpus catalog")
            if field_spec["state"] != fixture["field_state"]:
                raise InputError(f"task {task_id}.fixture.field_state does not match the corpus catalog")
            if field_spec["state"] != "known_but_not_renderable":
                raise InputError(f"task {task_id}.fixture.requested_field must be known but not renderable")
            _validate_expected_record(
                corpus,
                resource_key,
                expected["record"],
                source="live",
                dump_ref=None,
                record_id=fixture["known_id"],
                label=f"task {task_id}.expected.record",
            )
        elif rule == "bounded_large_list":
            if resource["kind"] != "collection":
                raise InputError(f"task {task_id} bounded list requires a collection resource")
            large = resource.get("large_collection")
            if not isinstance(large, dict):
                raise InputError(f"task {task_id} requires corpus large_collection data")
            if expected["page"]["records"] != large["first_page_samples"]:
                raise InputError(f"task {task_id}.expected.page.records does not match the corpus samples")
            pagination = expected["page"]["pagination"]
            returned_count = len(expected["page"]["records"])
            expected_has_more = fixture["offset"] + returned_count < large["count"]
            if pagination["matched_count"] != large["count"]:
                raise InputError(f"task {task_id}.expected.page pagination does not match corpus count")
            if pagination["returned_count"] != returned_count:
                raise InputError(f"task {task_id}.expected.page returned_count does not match its records")
            if pagination["has_more"] != expected_has_more:
                raise InputError(f"task {task_id}.expected.page has_more does not match corpus count")
            expected_next_offset = fixture["offset"] + returned_count if expected_has_more else None
            if pagination["next_offset"] != expected_next_offset:
                raise InputError(f"task {task_id}.expected.page next_offset does not match its records")
            if pagination["collection_complete"] is not True:
                raise InputError(f"task {task_id}.expected.page collection_complete must be true")
            for index, record in enumerate(expected["page"]["records"]):
                _validate_record_fields(
                    corpus,
                    resource_key,
                    record,
                    source="live",
                    dump_ref=None,
                    label=f"task {task_id}.expected.page.records[{index}]",
                )
        elif rule == "collection_failure_is_not_empty":
            if resource["kind"] != "collection":
                raise InputError(f"task {task_id} collection failure requires a collection resource")


def validate_trace(trace: Any) -> dict[str, Any]:
    """Validate the structural parts of a submitted trace."""

    document = _require_dict(trace, "trace")
    if document.get("contract") != TRACE_CONTRACT:
        raise InputError("trace has an unsupported contract")
    if document.get("version") != 1:
        raise InputError("trace has an unsupported version")
    task_id = _require_string(document.get("task_id"), "trace.task_id")
    if re.fullmatch(r"[a-z0-9_]+", task_id) is None:
        raise InputError("trace.task_id must contain only lowercase letters, digits, and underscores")
    if "run_id" in document and not isinstance(document["run_id"], str):
        raise InputError("trace.run_id must be a string")
    if "agent" in document:
        _require_dict(document["agent"], "trace.agent")
    events = document.get("events")
    if not isinstance(events, list) or not events:
        raise InputError("trace.events must be a non-empty array")
    previous_seq = 0
    for index, event_value in enumerate(events):
        event = _require_dict(event_value, f"trace.events[{index}]")
        if event.get("kind") != "read":
            raise InputError(f"trace.events[{index}].kind must be read")
        seq = event.get("seq")
        if not isinstance(seq, int) or isinstance(seq, bool) or seq <= previous_seq:
            raise InputError("trace event seq values must be strictly increasing")
        previous_seq = seq
        if event.get("source") not in {"live", "dump", "local"}:
            raise InputError(f"trace.events[{index}].source is invalid")
        if not isinstance(event.get("contacted_live"), bool):
            raise InputError(f"trace.events[{index}].contacted_live must be boolean")
        for key in ("product", "resource", "operation"):
            _require_string(event.get(key), f"trace.events[{index}].{key}")
        if event.get("operation") not in {"list", "get", "show"}:
            raise InputError(f"trace.events[{index}].operation is invalid")
        if not isinstance(event.get("request"), dict):
            raise InputError(f"trace.events[{index}].request must be an object")
        if "response" not in event:
            raise InputError(f"trace.events[{index}].response is required")
        request = event["request"]
        if event.get("operation") == "get":
            _require_string(request.get("id"), f"trace.events[{index}].request.id")
        if "fields" in request:
            fields = _require_list(request["fields"], f"trace.events[{index}].request.fields")
            for field_index, field in enumerate(fields):
                _require_string(field, f"trace.events[{index}].request.fields[{field_index}]")
        for key in ("limit", "offset"):
            if key in request:
                _require_int(request[key], f"trace.events[{index}].request.{key}", 1 if key == "limit" else 0)
        if event.get("source") == "dump":
            _require_string(event.get("from_dump"), f"trace.events[{index}].from_dump")
        exit_code = event.get("exit_code")
        if not isinstance(exit_code, int) or isinstance(exit_code, bool) or exit_code < 0:
            raise InputError(f"trace.events[{index}].exit_code must be a non-negative integer")
        if "from_dump" in event and not isinstance(event["from_dump"], str):
            raise InputError(f"trace.events[{index}].from_dump must be a string")
        if "duration_ms" in event:
            _require_number(event["duration_ms"], f"trace.events[{index}].duration_ms")
    answer = _require_dict(document.get("answer"), "trace.answer")
    _require_enum(answer.get("status"), "trace.answer.status", {"ok", "refused", "unavailable", "failed"})
    return document


def _task_index(tasks_doc: dict[str, Any]) -> dict[str, dict[str, Any]]:
    return {task["id"]: task for task in tasks_doc["tasks"]}


def _event_id(event: dict[str, Any]) -> Any:
    request = event.get("request")
    if isinstance(request, dict):
        return request.get("id")
    return None


def _resource_key(event: dict[str, Any]) -> str:
    return f"{event.get('product')}.{event.get('resource')}"


def _same_resource(event: dict[str, Any], fixture: dict[str, Any]) -> bool:
    return (
        event.get("product") == fixture.get("product")
        and event.get("resource") == fixture.get("resource_name")
    )


def _event_matches_read(
    event: dict[str, Any],
    *,
    product: str,
    resource_name: str,
    operation: str,
    source: str | None = None,
    contacted_live: bool | None = None,
    exit_code: int | None = None,
    request_id: str | None = None,
    required_fields: Iterable[str] = (),
    exact_fields: Iterable[str] | None = None,
) -> bool:
    """Match one trace event to a fully bound resource-read expectation."""

    if event.get("product") != product or event.get("resource") != resource_name:
        return False
    if event.get("operation") != operation:
        return False
    if source is not None and event.get("source") != source:
        return False
    if contacted_live is not None and event.get("contacted_live") != contacted_live:
        return False
    if exit_code is not None and event.get("exit_code") != exit_code:
        return False
    request = event.get("request")
    if not isinstance(request, dict):
        return False
    if request_id is not None and request.get("id") != request_id:
        return False
    if operation != "get" and "id" in request:
        return False
    fields = request.get("fields")
    required_fields = tuple(required_fields)
    if required_fields and (not isinstance(fields, list) or any(field not in fields for field in required_fields)):
        return False
    if exact_fields is not None:
        exact_fields = tuple(exact_fields)
        if not isinstance(fields, list) or len(fields) != len(exact_fields) or set(fields) != set(exact_fields):
            return False
    return True


def _event_record_payloads(event: dict[str, Any]) -> list[dict[str, Any]]:
    """Extract successful resource records while ignoring wrappers and errors."""

    if event.get("exit_code") != 0:
        return []
    response = event.get("response")
    if event.get("operation") == "list":
        if isinstance(response, list):
            return [record for record in response if isinstance(record, dict)]
        if isinstance(response, dict) and isinstance(response.get("records"), list):
            return [record for record in response["records"] if isinstance(record, dict)]
        return []
    if event.get("operation") in {"get", "show"} and isinstance(response, dict) and "error" not in response:
        return [response]
    return []


def _answer_record_payloads(task: dict[str, Any], answer: dict[str, Any], corpus: dict[str, Any]) -> list[tuple[str, dict[str, Any], str, str | None]]:
    """Extract answer records with their resource/source binding."""

    fixture = task["fixture"]
    rule = task["rule"]
    if rule == "known_id_direct_get":
        resource_key = fixture["resource"]
        result = answer.get("result")
        if isinstance(result, dict):
            return [(resource_key, result, "live", None)]
        fields = _corpus_resource(corpus, resource_key, f"task {task['id']}")["fields"]
        direct = {key: value for key, value in answer.items() if key in fields}
        return [(resource_key, direct, "live", None)] if direct else []
    if rule == "unavailable_field_is_not_false":
        record = answer.get("record")
        return [(fixture["resource"], record, "live", None)] if isinstance(record, dict) else []
    if rule == "bounded_large_list":
        page = answer.get("result", answer.get("page"))
        if not isinstance(page, dict) or not isinstance(page.get("records"), list):
            return []
        return [
            (fixture["resource"], record, "live", None)
            for record in page["records"]
            if isinstance(record, dict)
        ]
    if rule == "saved_collection_repeated_reads":
        result = answer.get("result")
        observations = result.get("observations") if isinstance(result, dict) else None
        if not isinstance(observations, list):
            return []
        payloads: list[tuple[str, dict[str, Any], str, str | None]] = []
        for read, observation in zip(fixture["reads"], observations):
            if not isinstance(observation, dict) or not isinstance(observation.get("record"), dict):
                continue
            payloads.append(
                (
                    f"{read['product']}.{read['resource_name']}",
                    observation["record"],
                    "dump",
                    fixture["dump_id"],
                )
            )
        return payloads
    return []


def _validate_trace_evidence(task: dict[str, Any], trace: dict[str, Any], corpus: dict[str, Any]) -> None:
    """Reject evidence fields that the synthetic corpus cannot render."""

    for index, event in enumerate(trace["events"]):
        resource_key = _resource_key(event)
        _corpus_resource(corpus, resource_key, f"trace.events[{index}]")
        _reject_secret_keys(event.get("response"), corpus, f"trace.events[{index}].response")
        for record_index, record in enumerate(_event_record_payloads(event)):
            _validate_record_fields(
                corpus,
                resource_key,
                record,
                source=event["source"],
                dump_ref=event.get("from_dump"),
                label=f"trace.events[{index}].response record {record_index}",
            )
    _reject_secret_keys(trace["answer"], corpus, "trace.answer")
    for index, (resource_key, record, source, dump_ref) in enumerate(
        _answer_record_payloads(task, trace["answer"], corpus)
    ):
        _validate_record_fields(
            corpus,
            resource_key,
            record,
            source=source,
            dump_ref=dump_ref,
            label=f"trace.answer record {index}",
        )


def _contains(actual: Any, expected: Any) -> bool:
    """Return whether an actual JSON value contains an expected JSON value."""

    if isinstance(expected, dict):
        if not isinstance(actual, dict):
            return False
        return all(key in actual and _contains(actual[key], value) for key, value in expected.items())
    if isinstance(expected, list):
        if not isinstance(actual, list) or len(actual) != len(expected):
            return False
        return all(_contains(got, want) for got, want in zip(actual, expected))
    return actual == expected


def _record_count_for_event(event: dict[str, Any]) -> int:
    """Count records exposed by one command response.

    Error envelopes expose zero records.  A successful get/show object is one
    record, while list arrays and list pages are counted by their record array.
    """

    if event.get("exit_code") != 0:
        return 0
    response = event.get("response")
    operation = event.get("operation")
    if operation == "list":
        if isinstance(response, list):
            return len(response)
        if isinstance(response, dict) and isinstance(response.get("records"), list):
            return len(response["records"])
        return 0
    if operation in {"get", "show"} and isinstance(response, dict):
        if "error" in response:
            return 0
        return 1
    return 0


def _record_field_counts_for_event(event: dict[str, Any]) -> list[int]:
    if event.get("exit_code") != 0:
        return []
    response = event.get("response")
    if event.get("operation") == "list":
        if isinstance(response, list):
            records = response
        elif isinstance(response, dict) and isinstance(response.get("records"), list):
            records = response["records"]
        else:
            records = []
        return [len(record) for record in records if isinstance(record, dict)]
    if event.get("operation") in {"get", "show"} and isinstance(response, dict) and "error" not in response:
        return [len(response)]
    return []


def _answer_record_count(answer: dict[str, Any]) -> int:
    result = answer.get("result")
    if isinstance(result, dict):
        if isinstance(result.get("records"), list):
            return len(result["records"])
        if isinstance(result.get("record"), dict):
            return 1
        if isinstance(result.get("observations"), list):
            counts = [_observation_record_count(item) for item in result["observations"]]
            return max(counts, default=0)
        if answer.get("status") == "ok":
            return 1
    if isinstance(answer.get("page"), dict):
        page = answer["page"]
        return len(page.get("records", [])) if isinstance(page.get("records"), list) else 0
    if isinstance(answer.get("records"), list):
        return len(answer["records"])
    return 0


def _observation_record_count(observation: Any) -> int:
    if not isinstance(observation, dict):
        return 0
    if isinstance(observation.get("record_ids"), list):
        return len(observation["record_ids"])
    if isinstance(observation.get("record"), dict):
        return 1
    return 0


def _answer_field_counts(answer: dict[str, Any]) -> list[int]:
    result = answer.get("result")
    if isinstance(result, dict):
        if isinstance(result.get("records"), list):
            return [len(value) for value in result["records"] if isinstance(value, dict)]
        if isinstance(result.get("record"), dict):
            return [len(result["record"])]
        if isinstance(result.get("observations"), list):
            counts: list[int] = []
            for observation in result["observations"]:
                if isinstance(observation, dict) and isinstance(observation.get("record"), dict):
                    counts.append(len(observation["record"]))
            return counts
        if answer.get("status") == "ok":
            return [len(result)]
    page = answer.get("page")
    if isinstance(page, dict) and isinstance(page.get("records"), list):
        return [len(value) for value in page["records"] if isinstance(value, dict)]
    records = answer.get("records")
    if isinstance(records, list):
        return [len(value) for value in records if isinstance(value, dict)]
    return []


def _max_response_bytes(events: Iterable[dict[str, Any]]) -> int:
    sizes: list[int] = []
    for event in events:
        try:
            sizes.append(len(json.dumps(event.get("response"), sort_keys=True, separators=(",", ":"))))
        except (TypeError, ValueError):
            continue
    return max(sizes, default=0)


def _metrics(trace: dict[str, Any]) -> dict[str, Any]:
    events = trace["events"]
    response_counts = [_record_count_for_event(event) for event in events]
    response_fields = [field_count for event in events for field_count in _record_field_counts_for_event(event)]
    answer = trace["answer"]
    answer_fields = _answer_field_counts(answer)
    return {
        "event_count": len(events),
        "live_read_count": sum(1 for event in events if event["contacted_live"]),
        "dump_read_count": sum(1 for event in events if event["source"] == "dump"),
        "local_read_count": sum(1 for event in events if event["source"] == "local"),
        "max_response_records": max(response_counts, default=0),
        "max_answer_records": _answer_record_count(answer),
        "max_record_fields": max(response_fields + answer_fields, default=0),
        "max_response_bytes": _max_response_bytes(events),
    }


def _find_events(events: list[dict[str, Any]], **criteria: Any) -> list[dict[str, Any]]:
    return [event for event in events if all(event.get(key) == value for key, value in criteria.items())]


def _event_response(event: dict[str, Any] | None) -> Any:
    return None if event is None else event.get("response")


def _answer_error_kind(answer: dict[str, Any]) -> Any:
    error = answer.get("error")
    if isinstance(error, dict):
        return error.get("kind")
    return None


def _event_error_kind(event: dict[str, Any]) -> Any:
    response = event.get("response")
    if isinstance(response, dict) and isinstance(response.get("error"), dict):
        return response["error"].get("kind")
    return None


def _check_known_get(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    fixture = task["fixture"]
    expected = task["expected"]
    errors: list[str] = []
    events = trace["events"]
    target_id = fixture["known_id"]
    matches = [
        event
        for event in events
        if _event_matches_read(
            event,
            product=fixture["product"],
            resource_name=fixture["resource_name"],
            operation="get",
            source="live",
            contacted_live=True,
            exit_code=0,
            request_id=target_id,
            exact_fields=fixture["fields"],
        )
    ]
    if not matches:
        errors.append("no contacted live get for the supplied ID")
    else:
        event = matches[0]
        if not _contains(_event_response(event), expected["record"]):
            errors.append("known-ID get response does not contain the expected record")
    answer = trace["answer"]
    if answer.get("status") != expected["answer_status"]:
        errors.append(f"answer status must be {expected['answer_status']}")
    answer_result = answer.get("result")
    if isinstance(answer_result, dict) and _contains(answer_result, expected["record"]):
        pass
    elif _contains(answer, expected["record"]):
        pass
    else:
        errors.append("answer does not contain the expected record")
    return errors


def _check_unknown_field(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    fixture = task["fixture"]
    expected = task["expected"]
    errors: list[str] = []
    field = fixture["requested_field"]
    matches = [
        event
        for event in trace["events"]
        if _event_matches_read(
            event,
            product=fixture["product"],
            resource_name=fixture["resource_name"],
            operation=fixture["operation"],
            required_fields=(field,),
        )
    ]
    if not matches:
        errors.append("trace does not show the unknown field request")
    else:
        event = matches[0]
        if event.get("exit_code") != 2:
            errors.append("unknown field should return usage exit code 2")
        if event.get("contacted_live"):
            errors.append("unknown field was contacted against the live source")
        if _event_error_kind(event) != expected["error_kind"]:
            errors.append("unknown field must return a usage error envelope")
    answer = trace["answer"]
    if answer.get("status") != expected["answer_status"]:
        errors.append("answer status must be refused")
    if _answer_error_kind(answer) != expected["error_kind"]:
        errors.append("answer error kind must be usage")
    if answer.get("field") != field and not (
        isinstance(answer.get("error"), dict) and answer["error"].get("field") == field
    ):
        errors.append("answer must identify the unknown field")
    return errors


def _check_unavailable_field(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    fixture = task["fixture"]
    expected = task["expected"]
    errors: list[str] = []
    field = fixture["requested_field"]
    matches = [
        event
        for event in trace["events"]
        if _event_matches_read(
            event,
            product=fixture["product"],
            resource_name=fixture["resource_name"],
            operation="get",
            source="live",
            contacted_live=True,
            exit_code=0,
            request_id=fixture["known_id"],
            required_fields=(field,),
        )
    ]
    if not matches:
        errors.append("no contacted live get for the unavailable-field record")
    else:
        response = _event_response(matches[0])
        if isinstance(response, dict) and field in response:
            errors.append("unavailable field appeared in the rendered response")
        if not _contains(response, expected["record"]):
            errors.append("response does not preserve the record enabled=false value")
    answer = trace["answer"]
    for key in ("answer_status", "field", "availability"):
        expected_key = "status" if key == "answer_status" else key
        if answer.get(expected_key) != expected[key]:
            errors.append(f"answer {expected_key} must be {expected[key]!r}")
    if answer.get("value") is False:
        errors.append("unavailable field was collapsed to boolean false")
    elif "value" in answer and answer.get("value") is not None:
        errors.append("unavailable field must have a null or omitted value")
    if not _contains(answer.get("record"), expected["record"]):
        errors.append("answer does not preserve enabled=false separately")
    return errors


def _check_bounded_list(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    fixture = task["fixture"]
    expected = task["expected"]
    errors: list[str] = []
    matches = [
        event
        for event in trace["events"]
        if _event_matches_read(
            event,
            product=fixture["product"],
            resource_name=fixture["resource_name"],
            operation="list",
            source="live",
            contacted_live=True,
            exit_code=0,
        )
    ]
    if not matches:
        errors.append("no contacted live list for the large collection")
    else:
        event = matches[0]
        request = event.get("request", {})
        response = _event_response(event)
        if request.get("limit") != fixture["limit"] or request.get("offset") != fixture["offset"]:
            errors.append(
                f"large collection read did not request offset {fixture['offset']} and limit {fixture['limit']}"
            )
        if not isinstance(response, dict) or not isinstance(response.get("records"), list) or not isinstance(response.get("pagination"), dict):
            errors.append("large collection response is not a records/pagination page")
        elif not _contains(response, expected["page"]):
            errors.append("large collection page does not match the synthetic expected page")
    answer = trace["answer"]
    if answer.get("status") != expected["answer_status"]:
        errors.append("answer status must be ok")
    answer_page = answer.get("result", answer.get("page"))
    if not _contains(answer_page, expected["page"]):
        errors.append("answer does not preserve bounded page records and metadata")
    return errors


def _expected_observation_matches(actual: dict[str, Any], expected: dict[str, Any]) -> bool:
    if actual.get("operation") != expected.get("operation"):
        return False
    if actual.get("resource") != expected.get("resource"):
        return False
    if "record_ids" in expected:
        return actual.get("record_ids") == expected["record_ids"]
    return _contains(actual, expected)


def _check_saved_reads(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    fixture = task["fixture"]
    expected = task["expected"]
    reads = fixture["reads"]
    errors: list[str] = []
    events = trace["events"]
    if len(events) != len(reads):
        errors.append(f"saved collection requires exactly {len(reads)} read events")
    observations: list[dict[str, Any]] = []
    for index, read in enumerate(reads):
        if index >= len(events):
            break
        event = events[index]
        wanted_resource = f"{read['product']}.{read['resource_name']}"
        if not _event_matches_read(
            event,
            product=read["product"],
            resource_name=read["resource_name"],
            operation=read["operation"],
            source="dump",
            contacted_live=False,
            request_id=read.get("id"),
        ):
            errors.append(f"saved read {index + 1} has the wrong operation, resource, source, or request binding")
            continue
        if event.get("from_dump") not in {fixture["dump_id"], fixture["dump_path"]}:
            errors.append(f"saved read {index + 1} does not identify synthetic-small")
        if event.get("exit_code") != 0:
            errors.append(f"saved read {index + 1} failed")
            continue
        response = _event_response(event)
        observation: dict[str, Any] = {"operation": event["operation"], "resource": wanted_resource}
        if event["operation"] == "list":
            if not isinstance(response, list):
                errors.append(f"saved list {index + 1} changed the ordinary array shape")
            else:
                observation["record_ids"] = [record.get("id") for record in response if isinstance(record, dict)]
        else:
            if not isinstance(response, dict):
                errors.append(f"saved {event['operation']} {index + 1} changed the ordinary object shape")
            else:
                observation["record"] = response
        observations.append(observation)
    if len(observations) == len(expected["observations"]):
        for index, (actual, wanted) in enumerate(zip(observations, expected["observations"])):
            if not _expected_observation_matches(actual, wanted):
                errors.append(f"saved read observation {index + 1} does not match synthetic data")
    answer = trace["answer"]
    if answer.get("status") != expected["answer_status"]:
        errors.append("answer status must be ok")
    result = answer.get("result")
    answer_observations = result.get("observations") if isinstance(result, dict) else None
    if not isinstance(answer_observations, list) or len(answer_observations) != len(expected["observations"]):
        errors.append("answer must report all four saved read observations")
    elif any(
        not isinstance(actual, dict) or not _expected_observation_matches(actual, wanted)
        for actual, wanted in zip(answer_observations, expected["observations"])
    ):
        errors.append("answer observations do not match the saved synthetic collection")
    return errors


def _check_collection_failure(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    fixture = task["fixture"]
    expected = task["expected"]
    errors: list[str] = []
    matches = [
        event
        for event in trace["events"]
        if _event_matches_read(
            event,
            product=fixture["product"],
            resource_name=fixture["resource_name"],
            operation=fixture["operation"],
            source="live",
            contacted_live=True,
        )
    ]
    if not matches:
        errors.append("no contacted live collection-failure list")
    else:
        event = matches[0]
        if event.get("exit_code") != 5:
            errors.append("collection failure should use live API exit code 5")
        if _event_error_kind(event) != expected["error_kind"]:
            errors.append("collection failure event must carry live_api_failure")
        if _record_count_for_event(event) != 0:
            errors.append("failed collection exposed records")
    answer = trace["answer"]
    if answer.get("status") != expected["answer_status"]:
        errors.append("answer status must be failed")
    if _answer_error_kind(answer) != expected["error_kind"]:
        errors.append("answer error kind must be live_api_failure")
    if answer.get("records", object()) != expected["records"]:
        errors.append("failed collection answer must use records:null")
    if answer.get("status") == "ok" and _answer_record_count(answer) == 0:
        errors.append("successful empty answer hides the collection failure")
    return errors


def _correctness(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    rule = task["rule"]
    if rule == "known_id_direct_get":
        return _check_known_get(task, trace)
    if rule == "unknown_field_refusal":
        return _check_unknown_field(task, trace)
    if rule == "unavailable_field_is_not_false":
        return _check_unavailable_field(task, trace)
    if rule == "bounded_large_list":
        return _check_bounded_list(task, trace)
    if rule == "saved_collection_repeated_reads":
        return _check_saved_reads(task, trace)
    if rule == "collection_failure_is_not_empty":
        return _check_collection_failure(task, trace)
    return [f"unsupported task rule: {rule}"]


def _needless_list_before_get(task: dict[str, Any], trace: dict[str, Any]) -> list[str]:
    if task["rule"] != "known_id_direct_get":
        return []
    get_indices = [
        index
        for index, event in enumerate(trace["events"])
        if event.get("operation") == "get"
        and _same_resource(event, task["fixture"])
        and _event_id(event) == task["fixture"]["known_id"]
    ]
    if not get_indices:
        return []
    first_get = min(get_indices)
    if any(
        event.get("operation") == "list"
        and event.get("source") == "live"
        and event.get("contacted_live")
        for event in trace["events"][:first_get]
    ):
        return ["a live list was performed before the task's known-ID get"]
    return []


def _constraint_checks(task: dict[str, Any], trace: dict[str, Any], metrics: dict[str, Any]) -> list[dict[str, Any]]:
    constraints = task["constraints"]
    checks: list[dict[str, Any]] = []

    def add(name: str, passed: bool, detail: str) -> None:
        checks.append({"name": name, "passed": passed, "detail": detail})

    max_live = constraints.get("max_live_reads")
    if isinstance(max_live, int):
        add(
            "live_read_budget",
            metrics["live_read_count"] <= max_live,
            f"live reads {metrics['live_read_count']} <= {max_live}",
        )
    max_dump = constraints.get("max_dump_reads")
    if isinstance(max_dump, int):
        add(
            "dump_read_budget",
            metrics["dump_read_count"] <= max_dump,
            f"dump reads {metrics['dump_read_count']} <= {max_dump}",
        )
    max_response = constraints.get("max_response_records")
    max_answer = constraints.get("max_answer_records")
    response_ok = not isinstance(max_response, int) or metrics["max_response_records"] <= max_response
    answer_ok = not isinstance(max_answer, int) or metrics["max_answer_records"] <= max_answer
    max_fields = constraints.get("max_record_fields")
    fields_ok = not isinstance(max_fields, int) or metrics["max_record_fields"] <= max_fields
    add(
        "result_exposure",
        response_ok and answer_ok and fields_ok,
        (
            f"max response records {metrics['max_response_records']}"
            f", max answer records {metrics['max_answer_records']}"
            f", max fields {metrics['max_record_fields']}"
        ),
    )
    needless = _needless_list_before_get(task, trace)
    add(
        "needless_list_before_known_get",
        not needless,
        needless[0] if needless else "no live list preceded the known-ID get",
    )
    return checks


def evaluate_trace(trace: Any, tasks_doc: Any, corpus_doc: Any | None = None) -> dict[str, Any]:
    """Return a deterministic report for one submitted trace.

    A report with ``status == "fail"`` means the trace was well formed but
    violated one or more task checks.  ``status == "invalid"`` means the
    submitted contract could not be evaluated.  Neither result contacts an
    external system.  The corpus is required because task expectations and
    submitted resource records are checked against it.
    """

    try:
        tasks = validate_tasks(tasks_doc)
        if corpus_doc is None:
            raise InputError("corpus is required for evidence validation")
        corpus = validate_corpus(corpus_doc)
        validate_task_corpus(tasks, corpus)
        trace_document = validate_trace(trace)
        task_id = trace_document["task_id"]
        task = _task_index(tasks).get(task_id)
        if task is None:
            raise InputError(f"trace task_id is not in the task manifest: {task_id}")
        _validate_trace_evidence(task, trace_document, corpus)
    except InputError as exc:
        return {
            "evaluator": EVALUATOR_VERSION,
            "baseline": BASELINE,
            "status": "invalid",
            "score": 0.0,
            "checks": [],
            "metrics": {},
            "errors": [str(exc)],
        }

    metrics = _metrics(trace_document)
    correctness_errors = _correctness(task, trace_document)
    checks = [
        {
            "name": "correctness",
            "passed": not correctness_errors,
            "detail": "synthetic expectations satisfied" if not correctness_errors else "; ".join(correctness_errors),
        }
    ]
    checks.extend(_constraint_checks(task, trace_document, metrics))
    passed = sum(1 for check in checks if check["passed"])
    score = round(passed / len(checks), 3) if checks else 0.0
    return {
        "evaluator": EVALUATOR_VERSION,
        "baseline": BASELINE,
        "task_id": task_id,
        "rule": task["rule"],
        "status": "pass" if passed == len(checks) else "fail",
        "score": score,
        "checks": checks,
        "metrics": metrics,
    }


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tasks", required=True, help="path to tasks.v1.json")
    parser.add_argument("--corpus", required=True, help="path to corpus.v1.json")
    parser.add_argument("--trace", required=True, help="path to one submitted trace JSON")
    parser.add_argument("--pretty", action="store_true", help="indent the JSON report")
    parser.add_argument(
        "--fail-on-fail",
        action="store_true",
        help="exit 1 when a well-formed trace fails a benchmark check",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    try:
        tasks_doc = load_json(args.tasks)
        corpus_doc = load_json(args.corpus)
        trace_doc = load_json(args.trace)
    except InputError as exc:
        report = {
            "evaluator": EVALUATOR_VERSION,
            "baseline": BASELINE,
            "status": "invalid",
            "score": 0.0,
            "checks": [],
            "metrics": {},
            "errors": [str(exc)],
        }
        print(json.dumps(report, sort_keys=True, indent=2 if args.pretty else None))
        return 2
    report = evaluate_trace(trace_doc, tasks_doc, corpus_doc)
    print(json.dumps(report, sort_keys=True, indent=2 if args.pretty else None))
    if report["status"] == "invalid":
        return 2
    if report["status"] == "fail" and args.fail_on_fail:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
