export type CurveMode = "linear" | "smooth";

export interface PositionedChartPoint {
  x: number;
  y: number | null;
}

function coordinate(value: number): string {
  return value.toFixed(2);
}

function isValidPoint(point: PositionedChartPoint): point is { x: number; y: number } {
  return Number.isFinite(point.x) && point.y !== null && Number.isFinite(point.y);
}

export function validPointSegments(points: PositionedChartPoint[]): Array<Array<{ x: number; y: number }>> {
  const segments: Array<Array<{ x: number; y: number }>> = [];
  let segment: Array<{ x: number; y: number }> = [];

  for (const point of points) {
    if (!isValidPoint(point)) {
      if (segment.length > 0) segments.push(segment);
      segment = [];
      continue;
    }

    const previous = segment[segment.length - 1];
    if (previous && point.x <= previous.x) {
      segments.push(segment);
      segment = [];
    }

    segment.push(point);
  }

  if (segment.length > 0) segments.push(segment);

  return segments;
}

export function linearPath(points: Array<{ x: number; y: number }>): string {
  if (points.length === 0) return "";

  return points
    .map((point, index) => {
      const command = index === 0 ? "M" : "L";
      return `${command} ${coordinate(point.x)} ${coordinate(point.y)}`;
    })
    .join(" ");
}

export function smoothPath(points: Array<{ x: number; y: number }>): string {
  if (points.length < 3) return linearPath(points);

  const slopes: number[] = [];
  const distances: number[] = [];

  for (let index = 0; index < points.length - 1; index++) {
    const distance = points[index + 1].x - points[index].x;
    if (distance <= 0) return linearPath(points);

    distances.push(distance);
    slopes.push((points[index + 1].y - points[index].y) / distance);
  }

  const tangents = new Array(points.length).fill(0);
  tangents[0] = slopes[0];
  tangents[points.length - 1] = slopes[slopes.length - 1];

  for (let index = 1; index < points.length - 1; index++) {
    const previousSlope = slopes[index - 1];
    const nextSlope = slopes[index];

    if (previousSlope === 0 || nextSlope === 0 || Math.sign(previousSlope) !== Math.sign(nextSlope)) {
      tangents[index] = 0;
      continue;
    }

    const previousDistance = distances[index - 1];
    const nextDistance = distances[index];
    const previousWeight = 2 * nextDistance + previousDistance;
    const nextWeight = nextDistance + 2 * previousDistance;
    tangents[index] = (previousWeight + nextWeight) / (previousWeight / previousSlope + nextWeight / nextSlope);
  }

  const commands = [`M ${coordinate(points[0].x)} ${coordinate(points[0].y)}`];

  for (let index = 0; index < points.length - 1; index++) {
    const distance = distances[index];
    const current = points[index];
    const next = points[index + 1];
    const controlX1 = current.x + distance / 3;
    const controlY1 = current.y + (tangents[index] * distance) / 3;
    const controlX2 = next.x - distance / 3;
    const controlY2 = next.y - (tangents[index + 1] * distance) / 3;

    commands.push(
      `C ${coordinate(controlX1)} ${coordinate(controlY1)} ${coordinate(controlX2)} ${coordinate(controlY2)} ${coordinate(next.x)} ${coordinate(next.y)}`,
    );
  }

  return commands.join(" ");
}

export function chartPath(points: Array<{ x: number; y: number }>, curve: CurveMode): string {
  return curve === "smooth" ? smoothPath(points) : linearPath(points);
}

export function chartAreaPath(points: Array<{ x: number; y: number }>, baseline: number, curve: CurveMode): string {
  if (points.length < 2) return "";

  const path = chartPath(points, curve);
  const first = points[0];
  const last = points[points.length - 1];

  return `${path} L ${coordinate(last.x)} ${coordinate(baseline)} L ${coordinate(first.x)} ${coordinate(baseline)} Z`;
}
