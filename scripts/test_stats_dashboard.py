#!/usr/bin/env python3
"""
Exercise llama-swap through the OpenAI Python client and validate the Dashboard UI.

Dependencies:
  python -m pip install openai playwright
  python -m playwright install chromium

Example:
  python scripts/test_stats_dashboard.py \
    --base-url http://localhost:8080 \
    --models qwen2.5 smollm2 \
    --requests-per-model 2
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Any

try:
    from openai import OpenAI
except ImportError as exc:
    raise SystemExit("Missing dependency: python -m pip install openai") from exc


@dataclass(frozen=True)
class ModelTotals:
    requests: int
    new_input_tokens: int
    cached_tokens: int
    generated_tokens: int

    @property
    def total_input_tokens(self) -> int:
        return self.new_input_tokens + self.cached_tokens

    @property
    def total_tokens(self) -> int:
        return self.total_input_tokens + self.generated_tokens

    @property
    def cache_hit_rate(self) -> float:
        if self.total_input_tokens == 0:
            return 0.0
        return self.cached_tokens / self.total_input_tokens


def fetch_json(url: str, api_key: str | None = None) -> Any:
    headers = {"Accept": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"

    request = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(request, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def metric_id(metric: dict[str, Any]) -> int:
    value = metric.get("id")
    return value if isinstance(value, int) else -1


def int_field(metric: dict[str, Any], name: str) -> int:
    value = metric.get(name, 0)
    return value if isinstance(value, int) else 0


def metrics_url(base_url: str) -> str:
    return f"{base_url.rstrip('/')}/api/metrics"


def fetch_metrics(base_url: str, api_key: str | None) -> list[dict[str, Any]]:
    metrics = fetch_json(metrics_url(base_url), api_key)
    if metrics is None:
        return []
    if not isinstance(metrics, list):
        raise RuntimeError(f"expected /api/metrics to return a list or null, got {type(metrics).__name__}")
    return [metric for metric in metrics if isinstance(metric, dict)]


def list_models(base_url: str, api_key: str | None) -> list[str]:
    models = fetch_json(f"{base_url.rstrip('/')}/v1/models", api_key)
    result = []
    for model in models.get("data", []) if isinstance(models, dict) else models:
        if isinstance(model, dict) and not model.get("unlisted"):
            model_id = model.get("id")
            if isinstance(model_id, str) and model_id:
                result.append(model_id)
    return result


def send_requests(client: OpenAI, models: list[str], requests_per_model: int, max_tokens: int) -> None:
    prompts = [
        "In one short paragraph, explain why local model routing metrics are useful.",
        "List three concise tips for improving LLM inference throughput.",
        "Write a tiny JSON object with keys status and reason.",
    ]

    for model in models:
        for index in range(requests_per_model):
            prompt = prompts[index % len(prompts)]
            print(f"request model={model} index={index + 1}/{requests_per_model}")
            response = client.chat.completions.create(
                model=model,
                messages=[
                    {"role": "system", "content": "Answer briefly. Keep the response under 80 words."},
                    {"role": "user", "content": prompt},
                ],
                max_tokens=max_tokens,
                temperature=0,
                stream=False,
            )
            content = response.choices[0].message.content or ""
            print(f"  response chars={len(content)}")


def wait_for_new_metrics(
    base_url: str,
    api_key: str | None,
    previous_max_id: int,
    expected_count: int,
    timeout_seconds: int,
) -> list[dict[str, Any]]:
    deadline = time.time() + timeout_seconds
    last_seen: list[dict[str, Any]] = []

    while time.time() < deadline:
        metrics = fetch_metrics(base_url, api_key)
        new_metrics = [metric for metric in metrics if metric_id(metric) > previous_max_id]
        last_seen = new_metrics
        if len(new_metrics) >= expected_count:
            return new_metrics

        time.sleep(0.5)

    raise TimeoutError(f"timed out waiting for {expected_count} new metrics; saw {len(last_seen)}")


def summarize(metrics: list[dict[str, Any]]) -> tuple[ModelTotals, dict[str, ModelTotals]]:
    by_model: dict[str, list[dict[str, Any]]] = {}
    for metric in metrics:
        model = metric.get("model")
        if not isinstance(model, str) or not model:
            model = "unknown"
        by_model.setdefault(model, []).append(metric)

    def total(group: list[dict[str, Any]]) -> ModelTotals:
        return ModelTotals(
            requests=len(group),
            new_input_tokens=sum(max(0, int_field(metric, "new_input_tokens")) for metric in group),
            cached_tokens=sum(max(0, int_field(metric, "cache_tokens")) for metric in group),
            generated_tokens=sum(max(0, int_field(metric, "output_tokens")) for metric in group),
        )

    global_totals = total(metrics)
    model_totals = {model: total(group) for model, group in by_model.items()}
    return global_totals, model_totals


def assert_metric_shape(metrics: list[dict[str, Any]], expected_models: set[str]) -> None:
    missing_models = expected_models - {str(metric.get("model")) for metric in metrics}
    if missing_models:
        raise AssertionError(f"missing metrics for models: {', '.join(sorted(missing_models))}")

    required_fields = {
        "id",
        "timestamp",
        "model",
        "cache_tokens",
        "new_input_tokens",
        "output_tokens",
        "prompt_per_second",
        "tokens_per_second",
        "duration_ms",
    }
    for metric in metrics:
        missing = required_fields - metric.keys()
        if missing:
            raise AssertionError(f"metric id={metric.get('id')} missing fields: {sorted(missing)}")
        if int_field(metric, "new_input_tokens") < 0:
            raise AssertionError(f"metric id={metric.get('id')} has negative new_input_tokens")
        if int_field(metric, "output_tokens") < 0:
            raise AssertionError(f"metric id={metric.get('id')} has negative output_tokens")


def fmt_int(value: int) -> str:
    return f"{value:,}"


def validate_dashboard(
    base_url: str,
    ui_url: str | None,
    global_totals: ModelTotals,
    model_totals: dict[str, ModelTotals],
    headed: bool,
) -> None:
    try:
        from playwright.sync_api import expect, sync_playwright
    except ImportError as exc:
        raise SystemExit("Missing dependency: python -m pip install playwright && python -m playwright install chromium") from exc

    target_url = ui_url or f"{base_url.rstrip('/')}/ui/#/dashboard"
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=not headed)
        page = browser.new_page(viewport={"width": 1600, "height": 1000})
        page.goto(target_url, wait_until="domcontentloaded")

        expect(page.get_by_role("heading", name="Dashboard")).to_be_visible(timeout=15_000)
        expect(page.get_by_role("heading", name="Total Tokens")).to_be_visible()
        expect(page.get_by_text("Per-Model Consumption Breakdown")).to_be_visible()

        body = page.locator("body")
        expect(body).to_contain_text(fmt_int(global_totals.total_tokens), timeout=15_000)
        expect(body).to_contain_text(fmt_int(global_totals.total_input_tokens))
        expect(body).to_contain_text(fmt_int(global_totals.generated_tokens))

        for model, totals in model_totals.items():
            row = page.locator("tr", has_text=model).first
            expect(row).to_be_visible(timeout=15_000)
            expect(row).to_contain_text(fmt_int(totals.requests))
            expect(row).to_contain_text(fmt_int(totals.new_input_tokens))
            expect(row).to_contain_text(fmt_int(totals.cached_tokens))
            expect(row).to_contain_text(fmt_int(totals.generated_tokens))
            expect(row).to_contain_text(f"{totals.cache_hit_rate * 100:.1f}%")

        browser.close()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Validate llama-swap Dashboard with OpenAI client traffic.")
    parser.add_argument("--base-url", default=os.getenv("LLAMA_SWAP_URL", "http://localhost:8080"), help="llama-swap base URL")
    parser.add_argument("--api-key", default=os.getenv("OPENAI_API_KEY") or os.getenv("LLAMA_SWAP_API_KEY") or "not-needed")
    parser.add_argument("--models", nargs="*", help="model ids to exercise; defaults to the first available model")
    parser.add_argument("--model-count", type=int, default=3, help="number of discovered models to use when --models is omitted")
    parser.add_argument("--requests-per-model", type=int, default=2)
    parser.add_argument("--max-tokens", type=int, default=64)
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--ui-url", help="override dashboard URL; defaults to BASE/ui/#/dashboard")
    parser.add_argument("--skip-dashboard", action="store_true", help="validate API metrics only")
    parser.add_argument("--headed", action="store_true", help="show browser while validating dashboard")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    base_url = args.base_url.rstrip("/")
    client = OpenAI(base_url=f"{base_url}/v1", api_key=args.api_key)

    before = fetch_metrics(base_url, args.api_key)
    previous_max_id = max((metric_id(metric) for metric in before), default=-1)

    models = args.models
    if not models:
        models = list_models(base_url, args.api_key)[: args.model_count]
    if not models:
        raise SystemExit("No models provided and none discovered from /api/models/")

    expected_count = len(models) * args.requests_per_model
    print(f"base_url={base_url}")
    print(f"models={', '.join(models)}")
    print(f"previous_max_metric_id={previous_max_id}")

    send_requests(client, models, args.requests_per_model, args.max_tokens)

    new_metrics = wait_for_new_metrics(base_url, args.api_key, previous_max_id, expected_count, args.timeout)
    assert_metric_shape(new_metrics, set(models))
    all_metrics = fetch_metrics(base_url, args.api_key)
    global_totals, model_totals = summarize(all_metrics)

    print("metric totals:")
    print(f"  requests: {global_totals.requests}")
    print(f"  new input: {global_totals.new_input_tokens}")
    print(f"  cached: {global_totals.cached_tokens}")
    print(f"  generated: {global_totals.generated_tokens}")
    print(f"  total tokens: {global_totals.total_tokens}")
    for model, totals in sorted(model_totals.items()):
        print(
            "  "
            f"{model}: requests={totals.requests}, new_input={totals.new_input_tokens}, "
            f"cached={totals.cached_tokens}, generated={totals.generated_tokens}, total={totals.total_tokens}"
        )

    if not args.skip_dashboard:
        validate_dashboard(base_url, args.ui_url, global_totals, model_totals, args.headed)
        print("dashboard validation: ok")

    print("ok")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (urllib.error.URLError, TimeoutError, AssertionError, RuntimeError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
