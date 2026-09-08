"""Offline contract tests for the synthetic agent workflow evaluator."""

from __future__ import annotations

import copy
import contextlib
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path


HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import evaluate  # noqa: E402  (the directory name is intentionally not a package)


TASKS_PATH = HERE / "fixtures" / "tasks.v1.json"
CORPUS_PATH = HERE / "fixtures" / "corpus.v1.json"


def _event(
    seq: int,
    *,
    operation: str,
    request: dict,
    response,
    source: str = "live",
    contacted_live: bool = True,
    exit_code: int = 0,
    from_dump: str | None = None,
    product: str = "zia",
    resource: str = "locations",
) -> dict:
    event = {
        "seq": seq,
        "kind": "read",
        "source": source,
        "contacted_live": contacted_live,
        "product": product,
        "resource": resource,
        "operation": operation,
        "request": request,
        "exit_code": exit_code,
        "response": response,
    }
    if from_dump is not None:
        event["from_dump"] = from_dump
    return event


def _trace(task_id: str, events: list[dict], answer: dict) -> dict:
    return {
        "contract": evaluate.TRACE_CONTRACT,
        "version": 1,
        "task_id": task_id,
        "run_id": f"test-{task_id}",
        "events": events,
        "answer": answer,
    }


def _large_page() -> dict:
    records = [
        {"id": f"large-{index:04d}", "name": f"Synthetic Location {index:04d}", "country": "US"}
        for index in range(1, 6)
    ]
    return {
        "records": records,
        "pagination": {
            "offset": 0,
            "limit": 5,
            "matched_count": 2500,
            "returned_count": 5,
            "has_more": True,
            "next_offset": 5,
            "collection_complete": True,
        },
    }


def _valid_traces() -> dict[str, dict]:
    live_locations = [
        {"id": "loc-001", "name": "Northstar Lab", "country": "US", "enabled": True, "region": "east"},
        {"id": "loc-002", "name": "Southstar Lab", "country": "US", "enabled": False, "region": "west"},
        {"id": "loc-003", "name": "Moonbase Lab", "country": "CA", "enabled": True, "region": "north"},
    ]
    saved_locations = [
        {"id": "loc-001", "name": "Northstar Lab", "country": "US", "description": "Fictional northern test site"},
        {"id": "loc-002", "name": "Southstar Lab", "country": "US", "description": "Fictional southern test site"},
        {"id": "loc-003", "name": "Moonbase Lab", "country": "CA", "description": "Fictional lunar test site"},
    ]
    page = _large_page()
    observations = [
        {"operation": "list", "resource": "zia.locations", "record_ids": ["loc-001", "loc-002", "loc-003"]},
        {
            "operation": "get",
            "resource": "zia.locations",
            "record": saved_locations[1],
        },
        {
            "operation": "show",
            "resource": "zia.advanced-settings",
            "record": {"apiSessionTimeout": 30, "enableOffice365": True},
        },
        {"operation": "list", "resource": "zia.locations", "record_ids": ["loc-001", "loc-002", "loc-003"]},
    ]
    saved_events = [
        _event(1, operation="list", request={}, response=saved_locations, source="dump", contacted_live=False, from_dump="synthetic-small"),
        _event(2, operation="get", request={"id": "loc-002"}, response=saved_locations[1], source="dump", contacted_live=False, from_dump="synthetic-small"),
        _event(
            3,
            operation="show",
            request={},
            response={"apiSessionTimeout": 30, "enableOffice365": True},
            source="dump",
            contacted_live=False,
            from_dump="synthetic-small",
            resource="advanced-settings",
        ),
        _event(4, operation="list", request={}, response=saved_locations, source="dump", contacted_live=False, from_dump="synthetic-small"),
    ]
    return {
        "known_id_direct_get": _trace(
            "known_id_direct_get",
            [
                _event(
                    1,
                    operation="get",
                    request={"id": "loc-001", "fields": ["id", "name", "country", "enabled"]},
                    response={key: live_locations[0][key] for key in ("id", "name", "country", "enabled")},
                )
            ],
            {"status": "ok", "result": {key: live_locations[0][key] for key in ("id", "name", "country", "enabled")}},
        ),
        "unknown_field_refusal": _trace(
            "unknown_field_refusal",
            [
                _event(
                    1,
                    operation="list",
                    request={"fields": ["namme"]},
                    response={"error": {"kind": "usage", "field": "namme"}},
                    contacted_live=False,
                    exit_code=2,
                )
            ],
            {"status": "refused", "error": {"kind": "usage", "field": "namme"}},
        ),
        "unavailable_field_is_not_false": _trace(
            "unavailable_field_is_not_false",
            [
                _event(
                    1,
                    operation="get",
                    request={"id": "loc-002", "fields": ["id", "name", "enabled", "description"]},
                    response=live_locations[1],
                )
            ],
            {
                "status": "unavailable",
                "field": "description",
                "availability": "known_but_not_renderable",
                "value": None,
                "record": {"id": "loc-002", "enabled": False},
            },
        ),
        "bounded_large_list": _trace(
            "bounded_large_list",
            [_event(1, operation="list", request={"limit": 5, "offset": 0}, response=page)],
            {"status": "ok", "result": page},
        ),
        "saved_collection_repeated_reads": _trace(
            "saved_collection_repeated_reads",
            saved_events,
            {"status": "ok", "result": {"observations": observations}},
        ),
        "collection_failure_is_not_empty": _trace(
            "collection_failure_is_not_empty",
            [
                _event(
                    1,
                    operation="list",
                    request={},
                    response={"error": {"kind": "live_api_failure", "message": "synthetic upstream timeout"}},
                    exit_code=5,
                )
            ],
            {"status": "failed", "error": {"kind": "live_api_failure"}, "records": None},
        ),
    }


class EvaluatorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        with TASKS_PATH.open(encoding="utf-8") as handle:
            cls.tasks = json.load(handle)
        with CORPUS_PATH.open(encoding="utf-8") as handle:
            cls.corpus = json.load(handle)

    def test_fixture_envelopes_are_valid(self) -> None:
        evaluate.validate_tasks(self.tasks)
        evaluate.validate_corpus(self.corpus)

    def test_all_six_valid_traces_pass(self) -> None:
        traces = _valid_traces()
        self.assertEqual(set(traces), {task["id"] for task in self.tasks["tasks"]})
        for task_id, trace in traces.items():
            with self.subTest(task_id=task_id):
                report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
                self.assertEqual(report["status"], "pass", report)
                self.assertEqual(report["baseline"], evaluate.BASELINE)

    def test_known_id_list_before_get_is_penalized(self) -> None:
        trace = _valid_traces()["known_id_direct_get"]
        trace["events"].insert(
            0,
            _event(1, operation="list", request={}, response=[{"id": "loc-001"}]),
        )
        trace["events"][1]["seq"] = 2
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        checks = {check["name"]: check for check in report["checks"]}
        self.assertFalse(checks["needless_list_before_known_get"]["passed"])
        self.assertFalse(checks["live_read_budget"]["passed"])

    def test_unknown_field_live_contact_is_rejected(self) -> None:
        trace = _valid_traces()["unknown_field_refusal"]
        trace["events"][0]["contacted_live"] = True
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertFalse(next(check for check in report["checks"] if check["name"] == "correctness")["passed"])

    def test_unavailable_field_cannot_be_false(self) -> None:
        trace = _valid_traces()["unavailable_field_is_not_false"]
        trace["answer"]["value"] = False
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("collapsed to boolean false", report["checks"][0]["detail"])

    def test_unavailable_field_must_be_requested(self) -> None:
        trace = _valid_traces()["unavailable_field_is_not_false"]
        trace["events"][0]["request"]["fields"] = ["id", "name", "enabled"]
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("no contacted live get", report["checks"][0]["detail"])

    def test_secret_fields_are_rejected_in_event_and_answer_evidence(self) -> None:
        for location, mutate in (
            (
                "event",
                lambda trace: trace["events"][0]["response"].update({"credential_hint": "SECRET"}),
            ),
            (
                "answer",
                lambda trace: trace["answer"]["record"].update({"credential_hint": "SECRET"}),
            ),
            (
                "answer-top-level",
                lambda trace: trace["answer"].update({"credential_hint": "SECRET"}),
            ),
        ):
            with self.subTest(location=location):
                trace = _valid_traces()["unavailable_field_is_not_false"]
                mutate(trace)
                report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
                self.assertEqual(report["status"], "invalid")
                self.assertTrue(any("secret field zia.locations.credential_hint" in error for error in report["errors"]))

    def test_record_values_are_bound_to_corpus(self) -> None:
        trace = _valid_traces()["unavailable_field_is_not_false"]
        trace["events"][0]["response"]["region"] = "wrong-region"
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "invalid")
        self.assertTrue(any("does not match the corpus record" in error for error in report["errors"]))

    def test_unavailable_field_requires_successful_get(self) -> None:
        trace = _valid_traces()["unavailable_field_is_not_false"]
        trace["events"][0]["exit_code"] = 5
        trace["events"][0]["response"] = {"error": {"kind": "live_api_failure"}}
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("no contacted live get", report["checks"][0]["detail"])

    def test_unavailable_field_requires_live_source(self) -> None:
        trace = _valid_traces()["unavailable_field_is_not_false"]
        trace["events"][0]["source"] = "local"
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("no contacted live get", report["checks"][0]["detail"])

    def test_bounded_page_requires_successful_list(self) -> None:
        trace = _valid_traces()["bounded_large_list"]
        trace["events"][0]["exit_code"] = 5
        trace["events"][0]["response"] = {"error": {"kind": "live_api_failure"}}
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("no contacted live list", report["checks"][0]["detail"])

    def test_large_list_overexposure_is_rejected(self) -> None:
        trace = _valid_traces()["bounded_large_list"]
        trace["events"][0]["response"] = [
            {"id": f"large-{index:04d}", "name": f"Synthetic Location {index:04d}", "country": "US"}
            for index in range(1, 2501)
        ]
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        exposure = next(check for check in report["checks"] if check["name"] == "result_exposure")
        self.assertFalse(exposure["passed"])

    def test_saved_collection_live_read_is_rejected(self) -> None:
        trace = _valid_traces()["saved_collection_repeated_reads"]
        trace["events"][2]["source"] = "live"
        trace["events"][2]["contacted_live"] = True
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertFalse(next(check for check in report["checks"] if check["name"] == "correctness")["passed"])
        self.assertFalse(next(check for check in report["checks"] if check["name"] == "live_read_budget")["passed"])

    def test_saved_get_binds_request_id_to_expected_read(self) -> None:
        trace = _valid_traces()["saved_collection_repeated_reads"]
        trace["events"][1]["request"]["id"] = "loc-001"
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("request binding", report["checks"][0]["detail"])

    def test_collection_failure_cannot_be_empty_success(self) -> None:
        trace = _valid_traces()["collection_failure_is_not_empty"]
        trace["events"][0] = _event(1, operation="list", request={}, response=[], exit_code=0)
        trace["answer"] = {"status": "ok", "result": {"records": []}}
        report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
        self.assertEqual(report["status"], "fail")
        self.assertIn("live API exit code 5", report["checks"][0]["detail"])

    def test_malformed_trace_is_invalid(self) -> None:
        trace = _valid_traces()["known_id_direct_get"]
        malformed = copy.deepcopy(trace)
        malformed["events"][0].pop("contacted_live")
        report = evaluate.evaluate_trace(malformed, self.tasks, self.corpus)
        self.assertEqual(report["status"], "invalid")
        self.assertEqual(report["baseline"], evaluate.BASELINE)

    def test_trace_missing_response_is_invalid(self) -> None:
        trace = _valid_traces()["known_id_direct_get"]
        malformed = copy.deepcopy(trace)
        malformed["events"][0].pop("response")
        report = evaluate.evaluate_trace(malformed, self.tasks, self.corpus)
        self.assertEqual(report["status"], "invalid")
        self.assertIn("response is required", report["errors"][0])

    def test_optional_trace_metadata_matches_published_types_and_bounds(self) -> None:
        valid = _valid_traces()["known_id_direct_get"]
        valid["run_id"] = "run-1"
        valid["agent"] = {"name": "synthetic-runner"}
        valid["events"][0]["duration_ms"] = 12.5
        report = evaluate.evaluate_trace(valid, self.tasks, self.corpus)
        self.assertEqual(report["status"], "pass", report)

        mutations = (
            ("run_id", 42, "trace.run_id"),
            ("agent", [], "trace.agent"),
            ("duration_ms", -1, "duration_ms"),
            ("duration_ms", "1", "duration_ms"),
            ("duration_ms", True, "duration_ms"),
            ("duration_ms", float("nan"), "duration_ms"),
            ("duration_ms", float("inf"), "duration_ms"),
        )
        for field, value, expected_error in mutations:
            with self.subTest(field=field, value=value):
                malformed = copy.deepcopy(_valid_traces()["known_id_direct_get"])
                if field == "duration_ms":
                    malformed["events"][0][field] = value
                else:
                    malformed[field] = value
                report = evaluate.evaluate_trace(malformed, self.tasks, self.corpus)
                self.assertEqual(report["status"], "invalid")
                self.assertTrue(any(expected_error in error for error in report["errors"]))

    def test_malformed_task_fixture_is_invalid_instead_of_keyerror(self) -> None:
        trace = _valid_traces()["known_id_direct_get"]
        malformed_tasks = copy.deepcopy(self.tasks)
        malformed_tasks["tasks"][0]["fixture"] = {}
        report = evaluate.evaluate_trace(trace, malformed_tasks, self.corpus)
        self.assertEqual(report["status"], "invalid")
        self.assertTrue(any("fixture.product" in error for error in report["errors"]))

    def test_known_get_requires_fixture_field_set(self) -> None:
        for fields in (None, ["id"]):
            with self.subTest(fields=fields):
                trace = _valid_traces()["known_id_direct_get"]
                if fields is None:
                    trace["events"][0]["request"].pop("fields")
                else:
                    trace["events"][0]["request"]["fields"] = fields
                report = evaluate.evaluate_trace(trace, self.tasks, self.corpus)
                self.assertEqual(report["status"], "fail")
                self.assertIn("no contacted live get", report["checks"][0]["detail"])

    def test_corpus_tamper_invalidates_task_expectation(self) -> None:
        corpus = copy.deepcopy(self.corpus)
        corpus["resources"]["zia.locations"]["records"][0]["name"] = "Tampered Lab"
        report = evaluate.evaluate_trace(_valid_traces()["known_id_direct_get"], self.tasks, corpus)
        self.assertEqual(report["status"], "invalid")
        self.assertTrue(any("does not match the corpus record" in error for error in report["errors"]))

    def test_task_contract_rejects_extra_top_level_and_task_fields(self) -> None:
        for location in ("top_level", "task"):
            with self.subTest(location=location):
                tasks = copy.deepcopy(self.tasks)
                if location == "top_level":
                    tasks["unexpected"] = True
                else:
                    tasks["tasks"][0]["unexpected"] = True
                report = evaluate.evaluate_trace(
                    _valid_traces()["known_id_direct_get"],
                    tasks,
                    self.corpus,
                )
                self.assertEqual(report["status"], "invalid")
                self.assertTrue(any("unsupported fields" in error for error in report["errors"]))

    def test_cli_returns_two_for_malformed_task_manifest(self) -> None:
        malformed_tasks = copy.deepcopy(self.tasks)
        malformed_tasks["tasks"][0]["fixture"] = {}
        trace_path = HERE / "examples" / "known-id-direct-get.trace.json"
        with tempfile.TemporaryDirectory() as temporary:
            task_path = Path(temporary) / "malformed-tasks.json"
            task_path.write_text(json.dumps(malformed_tasks), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()):
                status = evaluate.main(
                    [
                        "--tasks",
                        str(task_path),
                        "--corpus",
                        str(CORPUS_PATH),
                        "--trace",
                        str(trace_path),
                    ]
                )
        self.assertEqual(status, 2)


if __name__ == "__main__":
    unittest.main()
