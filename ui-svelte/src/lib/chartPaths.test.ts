import { describe, expect, test } from "vitest";
import { chartAreaPath, chartPath, smoothPath, validPointSegments } from "./chartPaths";

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
});
