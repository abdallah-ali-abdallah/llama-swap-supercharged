import { describe, expect, test } from "vitest";
import { chartAreaPath, chartPath, smoothPath, smoothedPointSegments, validPointSegments } from "./chartPaths";

describe("chartPaths", () => {
  test("keeps linear paths unchanged", () => {
    expect(
      chartPath(
        [
          { x: 0, y: 10 },
          { x: 20, y: 30 },
        ],
        "linear",
      ),
    ).toBe("M 0.00 10.00 L 20.00 30.00");
  });

  test("uses cubic commands for smooth paths with enough points", () => {
    const path = smoothPath([
      { x: 0, y: 10 },
      { x: 20, y: 30 },
      { x: 40, y: 20 },
    ]);

    expect(path).toContain(" C ");
    expect(path).not.toMatch(/NaN|Infinity/);
  });

  test("splits invalid and non-increasing points into separate drawable segments", () => {
    const segments = validPointSegments([
      { x: 0, y: 10 },
      { x: 20, y: 30 },
      { x: 30, y: null },
      { x: 40, y: 20 },
      { x: 40, y: 25 },
      { x: 60, y: 35 },
    ]);

    expect(segments).toEqual([
      [
        { x: 0, y: 10 },
        { x: 20, y: 30 },
      ],
      [{ x: 40, y: 20 }],
      [
        { x: 40, y: 25 },
        { x: 60, y: 35 },
      ],
    ]);
  });

  test("uses the selected curve mode for area paths", () => {
    const path = chartAreaPath(
      [
        { x: 0, y: 10 },
        { x: 20, y: 30 },
        { x: 40, y: 20 },
      ],
      50,
      "smooth",
    );

    expect(path).toContain(" C ");
    expect(path).toMatch(/ L 40\.00 50\.00 L 0\.00 50\.00 Z$/);
  });

  test("buckets close samples and smooths spike artifacts", () => {
    const segments = smoothedPointSegments(
      [
        { x: 0, y: 10 },
        { x: 2, y: 200 },
        { x: 3, y: 12 },
        { x: 30, y: 20 },
        { x: 60, y: 30 },
      ],
      { bucketSize: 5, windowSize: 3 },
    );

    expect(segments).toHaveLength(1);
    expect(segments[0]).toHaveLength(3);
    expect(segments[0][0].y).toBeLessThan(50);
    expect(segments[0][0].y).toBeGreaterThan(10);
    expect(segments[0].every((point) => Number.isFinite(point.x) && Number.isFinite(point.y))).toBe(true);
  });

  test("keeps null gaps while smoothing samples", () => {
    const segments = smoothedPointSegments(
      [
        { x: 0, y: 10 },
        { x: 2, y: 12 },
        { x: 5, y: null },
        { x: 8, y: 100 },
        { x: 9, y: 102 },
      ],
      { bucketSize: 4, windowSize: 3 },
    );

    expect(segments).toEqual([
      [{ x: 1, y: 11 }],
      [{ x: 8.5, y: 101 }],
    ]);
  });
});
