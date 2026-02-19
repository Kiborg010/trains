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

  const count = Math.floor(length / step);
  const ux = dx / length;
  const uy = dy / length;
  const slots = [];

  for (let i = 0; i <= count; i += 1) {
    slots.push({
      x: from.x + ux * step * i,
      y: from.y + uy * step * i,
    });
  }

  const last = slots[slots.length - 1];
  const isLastNearEnd =
    last && Math.hypot(last.x - to.x, last.y - to.y) < step * 0.25;

  if (!isLastNearEnd) {
    slots.push({ x: to.x, y: to.y });
  }

  return slots;
}
