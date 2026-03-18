export const GRID_SIZE = 40;

export function snap(value, grid = GRID_SIZE) {
  return Math.round(value / grid) * grid;
}

export function keyOf(x, y) {
  return `${x}:${y}`;
}

export function getSegmentSlots(segment, step = GRID_SIZE) {
  const { from, to } = segment;
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  const length = Math.hypot(dx, dy);

  if (length === 0) {
    return [{ x: from.x, y: from.y }];
  }

  const count = Math.max(1, Math.round(length / step));
  const ux = dx / length;
  const uy = dy / length;
  const actualStep = length / count;
  const slots = [];

  for (let i = 0; i <= count; i += 1) {
    const distance = actualStep * i;
    slots.push({
      x: from.x + ux * distance,
      y: from.y + uy * distance,
    });
  }
  slots[slots.length - 1] = { x: to.x, y: to.y };

  return slots;
}
