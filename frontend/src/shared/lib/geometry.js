export const GRID_SIZE = 20;

export function snap(value, grid = GRID_SIZE) {
  return Math.round(value / grid) * grid;
}

export function keyOf(x, y) {
  return `${x}:${y}`;
}
